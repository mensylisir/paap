package service

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"paap/internal/k8s"
	"paap/internal/model"

	"gopkg.in/yaml.v3"
	"gorm.io/gorm"
)

const defaultPlatformAddonBucket = "paap-charts"

type PlatformAddonArchiveSpec struct {
	Name         string                     `yaml:"name"`
	DisplayName  string                     `yaml:"displayName"`
	Category     string                     `yaml:"category"`
	Namespace    string                     `yaml:"namespace"`
	Version      string                     `yaml:"version"`
	InstallMode  string                     `yaml:"installMode"`
	S3Bucket     string                     `yaml:"s3Bucket"`
	S3Key        string                     `yaml:"s3Key"`
	DependsOn    []string                   `yaml:"dependsOn"`
	Capabilities []string                   `yaml:"capabilities"`
	Checks       k8s.PlatformAddonCheckSpec `yaml:"checks"`
	Description  string                     `yaml:"description"`
}

type PlatformAddonPackage struct {
	Spec      PlatformAddonArchiveSpec
	Readme    string
	Manifests []string
}

func ListPlatformAddons(db *gorm.DB) ([]model.ClusterAddon, error) {
	var addons []model.ClusterAddon
	if err := db.Order("category asc, name asc").Find(&addons).Error; err != nil {
		return nil, err
	}
	return addons, nil
}

func GetPlatformAddon(db *gorm.DB, name string) (model.ClusterAddon, error) {
	var addon model.ClusterAddon
	if err := db.Where("cluster_id = ? AND name = ?", 0, strings.TrimSpace(name)).First(&addon).Error; err != nil {
		return model.ClusterAddon{}, err
	}
	return addon, nil
}

func DisablePlatformAddon(db *gorm.DB, name string) (model.ClusterAddon, error) {
	addon, err := GetPlatformAddon(db, name)
	if err != nil {
		return model.ClusterAddon{}, err
	}
	addon.DesiredState = model.PlatformAddonDesiredDisabled
	addon.Status = model.PlatformAddonStatusDisabled
	addon.ErrorMessage = ""
	if err := db.Save(&addon).Error; err != nil {
		return model.ClusterAddon{}, err
	}
	return addon, nil
}

func DisablePlatformAddonFromArchive(ctx context.Context, db *gorm.DB, name string, archivePath string) (model.ClusterAddon, error) {
	addon, err := GetPlatformAddon(db, name)
	if err != nil {
		return model.ClusterAddon{}, err
	}
	pkg, err := ParsePlatformAddonArchive(archivePath)
	if err != nil {
		return addonWithFailure(db, addon, err)
	}
	if pkg.Spec.Name != addon.Name {
		return addonWithFailure(db, addon, fmt.Errorf("addon package name %q does not match requested addon %q", pkg.Spec.Name, addon.Name))
	}
	if err := k8s.DeletePlatformAddonManifests(ctx, pkg.Manifests); err != nil {
		addon.DesiredState = model.PlatformAddonDesiredDisabled
		return addonWithFailure(db, addon, err)
	}
	addon.DesiredState = model.PlatformAddonDesiredDisabled
	addon.Status = model.PlatformAddonStatusDisabled
	addon.ErrorMessage = ""
	addon.Conditions = "[]"
	addon.InstalledAt = nil
	now := time.Now()
	addon.LastCheckedAt = &now
	if err := db.Save(&addon).Error; err != nil {
		return model.ClusterAddon{}, err
	}
	return addon, nil
}

func EnablePlatformAddonFromArchive(ctx context.Context, db *gorm.DB, name string, archivePath string) (model.ClusterAddon, error) {
	addon, err := GetPlatformAddon(db, name)
	if err != nil {
		return model.ClusterAddon{}, err
	}
	pkg, err := ParsePlatformAddonArchive(archivePath)
	if err != nil {
		return addonWithFailure(db, addon, err)
	}
	if pkg.Spec.Name != addon.Name {
		return addonWithFailure(db, addon, fmt.Errorf("addon package name %q does not match requested addon %q", pkg.Spec.Name, addon.Name))
	}

	addon.DesiredState = model.PlatformAddonDesiredEnabled
	addon.Status = model.PlatformAddonStatusInstalling
	addon.ErrorMessage = ""
	if err := db.Save(&addon).Error; err != nil {
		return model.ClusterAddon{}, err
	}
	if err := k8s.ApplyPlatformAddonManifests(ctx, pkg.Manifests); err != nil {
		return addonWithFailure(db, addon, err)
	}

	refreshed := ClusterAddonFromPackage(pkg, addon.Source)
	copyPlatformAddonMetadata(&addon, refreshed)
	addon.DesiredState = model.PlatformAddonDesiredEnabled
	addon.Status = model.PlatformAddonStatusInstalling
	addon.ErrorMessage = ""
	now := time.Now()
	addon.InstalledAt = &now
	if err := db.Save(&addon).Error; err != nil {
		return model.ClusterAddon{}, err
	}
	return CheckAndSavePlatformAddon(ctx, db, addon.Name)
}

func CheckAndSavePlatformAddon(ctx context.Context, db *gorm.DB, name string) (model.ClusterAddon, error) {
	addon, err := GetPlatformAddon(db, name)
	if err != nil {
		return model.ClusterAddon{}, err
	}
	spec, err := platformAddonCheckSpecFromConfig(addon.Config)
	if err != nil {
		return model.ClusterAddon{}, err
	}
	status := k8s.CheckPlatformAddonStatus(ctx, spec)
	conditions, _ := json.Marshal(status.Conditions)
	now := time.Now()
	addon.Status = status.Status
	if addon.DesiredState == model.PlatformAddonDesiredDisabled && status.Status == model.PlatformAddonStatusUnavailable {
		addon.Status = model.PlatformAddonStatusDisabled
	}
	addon.Conditions = string(conditions)
	addon.LastCheckedAt = &now
	if status.Status == model.PlatformAddonStatusAvailable {
		addon.ErrorMessage = ""
	}
	if err := db.Save(&addon).Error; err != nil {
		return model.ClusterAddon{}, err
	}
	return addon, nil
}

func InstallPlatformAddonArchive(ctx context.Context, archivePath string) (PlatformAddonPackage, error) {
	pkg, err := ParsePlatformAddonArchive(archivePath)
	if err != nil {
		return PlatformAddonPackage{}, err
	}
	if err := k8s.ApplyPlatformAddonManifests(ctx, pkg.Manifests); err != nil {
		return PlatformAddonPackage{}, err
	}
	return pkg, nil
}

func UpsertPlatformAddonPackage(db *gorm.DB, pkg PlatformAddonPackage, source string) (model.ClusterAddon, error) {
	next := ClusterAddonFromPackage(pkg, source)
	addon := model.ClusterAddon{}
	err := db.Where("cluster_id = ? AND name = ?", next.ClusterID, next.Name).First(&addon).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return model.ClusterAddon{}, err
	}
	if err == gorm.ErrRecordNotFound {
		addon = next
	} else {
		copyPlatformAddonMetadata(&addon, next)
		if addon.DesiredState == "" {
			addon.DesiredState = model.PlatformAddonDesiredDisabled
		}
		if addon.Status == "" {
			addon.Status = model.PlatformAddonStatusUnknown
		}
		if addon.Conditions == "" {
			addon.Conditions = "[]"
		}
	}
	if err := db.Save(&addon).Error; err != nil {
		return model.ClusterAddon{}, err
	}
	return addon, nil
}

func SyncPlatformAddonPackagesFromDir(db *gorm.DB, rootDir string, source string) ([]model.ClusterAddon, error) {
	entries, err := os.ReadDir(rootDir)
	if err != nil {
		return nil, err
	}
	addons := make([]model.ClusterAddon, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pkg, err := ParsePlatformAddonDir(filepath.Join(rootDir, entry.Name()))
		if err != nil {
			return addons, err
		}
		addon, err := UpsertPlatformAddonPackage(db, pkg, source)
		if err != nil {
			return addons, err
		}
		addons = append(addons, addon)
	}
	return addons, nil
}

func ClusterAddonFromPackage(pkg PlatformAddonPackage, source string) model.ClusterAddon {
	if strings.TrimSpace(source) == "" {
		source = model.PlatformAddonSourceBuiltin
	}
	spec := normalizePlatformAddonSpec(pkg.Spec)
	if spec.S3Key == "" {
		spec.S3Key = defaultPlatformAddonS3Key(spec.Name, source)
	}
	dependsOn, _ := json.Marshal(spec.DependsOn)
	capabilities, _ := json.Marshal(spec.Capabilities)
	config, _ := json.Marshal(spec.Checks)
	return model.ClusterAddon{
		ClusterID:     0,
		Name:          spec.Name,
		DisplayName:   spec.DisplayName,
		Category:      firstNonEmpty(spec.Category, "platform"),
		Source:        source,
		Namespace:     spec.Namespace,
		Version:       spec.Version,
		InstallMode:   firstNonEmpty(spec.InstallMode, "manifest"),
		S3Bucket:      firstNonEmpty(spec.S3Bucket, defaultPlatformAddonBucket),
		S3Key:         spec.S3Key,
		DependsOn:     string(dependsOn),
		Capabilities:  string(capabilities),
		DesiredState:  model.PlatformAddonDesiredDisabled,
		Status:        model.PlatformAddonStatusUnknown,
		Config:        string(config),
		Conditions:    "[]",
		ErrorMessage:  "",
		Description:   spec.Description,
		Readme:        strings.TrimSpace(pkg.Readme),
		InstalledAt:   nil,
		LastCheckedAt: nil,
	}
}

func ParsePlatformAddonDir(dir string) (PlatformAddonPackage, error) {
	files := map[string]string{}
	err := filepath.WalkDir(dir, func(filePath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, filePath)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(filePath)
		if err != nil {
			return err
		}
		files[path.Clean(filepath.ToSlash(rel))] = string(data)
		return nil
	})
	if err != nil {
		return PlatformAddonPackage{}, fmt.Errorf("read platform addon dir: %w", err)
	}
	return parsePlatformAddonFiles(files)
}

func ParsePlatformAddonArchive(archivePath string) (PlatformAddonPackage, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return PlatformAddonPackage{}, fmt.Errorf("open platform addon archive: %w", err)
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		return PlatformAddonPackage{}, fmt.Errorf("read platform addon archive: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	files := map[string]string{}
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return PlatformAddonPackage{}, fmt.Errorf("read platform addon archive entry: %w", err)
		}
		if header.FileInfo().IsDir() {
			continue
		}
		clean := cleanAddonArchivePath(header.Name)
		if clean == "" {
			continue
		}
		body, err := io.ReadAll(tr)
		if err != nil {
			return PlatformAddonPackage{}, fmt.Errorf("read platform addon file %s: %w", header.Name, err)
		}
		files[clean] = string(body)
	}
	return parsePlatformAddonFiles(files)
}

func parsePlatformAddonFiles(files map[string]string) (PlatformAddonPackage, error) {
	files = normalizePlatformAddonFileRoot(files)
	rawSpec := strings.TrimSpace(files["addon.yaml"])
	if rawSpec == "" {
		return PlatformAddonPackage{}, fmt.Errorf("platform addon package must contain addon.yaml")
	}
	var spec PlatformAddonArchiveSpec
	if err := yaml.Unmarshal([]byte(rawSpec), &spec); err != nil {
		return PlatformAddonPackage{}, fmt.Errorf("parse addon.yaml: %w", err)
	}
	spec = normalizePlatformAddonSpec(spec)
	if spec.Name == "" || spec.DisplayName == "" {
		return PlatformAddonPackage{}, fmt.Errorf("addon.yaml must declare name and displayName")
	}
	manifestNames := make([]string, 0)
	for name := range files {
		if strings.HasPrefix(name, "manifests/") && (strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml")) {
			manifestNames = append(manifestNames, name)
		}
	}
	sort.Strings(manifestNames)
	manifests := make([]string, 0, len(manifestNames))
	for _, name := range manifestNames {
		body := strings.TrimSpace(files[name])
		if body != "" {
			manifests = append(manifests, body+"\n")
		}
	}
	if len(manifests) == 0 {
		return PlatformAddonPackage{}, fmt.Errorf("platform addon %s has no manifests", spec.Name)
	}
	return PlatformAddonPackage{
		Spec:      spec,
		Readme:    files["README.md"],
		Manifests: manifests,
	}, nil
}

func normalizePlatformAddonSpec(spec PlatformAddonArchiveSpec) PlatformAddonArchiveSpec {
	spec.Name = strings.TrimSpace(spec.Name)
	spec.DisplayName = strings.TrimSpace(spec.DisplayName)
	spec.Category = strings.TrimSpace(spec.Category)
	spec.Namespace = strings.TrimSpace(spec.Namespace)
	spec.Version = strings.TrimSpace(spec.Version)
	spec.InstallMode = firstNonEmpty(spec.InstallMode, "manifest")
	spec.S3Bucket = strings.TrimSpace(spec.S3Bucket)
	spec.S3Key = strings.TrimSpace(spec.S3Key)
	spec.Description = strings.TrimSpace(spec.Description)
	spec.DependsOn = compactStringList(spec.DependsOn)
	spec.Capabilities = compactStringList(spec.Capabilities)
	return spec
}

func normalizePlatformAddonFileRoot(files map[string]string) map[string]string {
	if _, ok := files["addon.yaml"]; ok {
		return files
	}
	prefixes := map[string]bool{}
	for name := range files {
		if strings.HasSuffix(name, "/addon.yaml") {
			prefixes[strings.TrimSuffix(name, "addon.yaml")] = true
		}
	}
	if len(prefixes) != 1 {
		return files
	}
	var prefix string
	for item := range prefixes {
		prefix = item
	}
	out := map[string]string{}
	for name, body := range files {
		if strings.HasPrefix(name, prefix) {
			out[strings.TrimPrefix(name, prefix)] = body
		}
	}
	return out
}

func copyPlatformAddonMetadata(dst *model.ClusterAddon, src model.ClusterAddon) {
	dst.ClusterID = src.ClusterID
	dst.Name = src.Name
	dst.DisplayName = src.DisplayName
	dst.Category = src.Category
	dst.Source = src.Source
	dst.Namespace = src.Namespace
	dst.Version = src.Version
	dst.InstallMode = src.InstallMode
	dst.S3Bucket = src.S3Bucket
	dst.S3Key = src.S3Key
	dst.DependsOn = src.DependsOn
	dst.Capabilities = src.Capabilities
	dst.Config = src.Config
	dst.Description = src.Description
	dst.Readme = src.Readme
}

func addonWithFailure(db *gorm.DB, addon model.ClusterAddon, err error) (model.ClusterAddon, error) {
	addon.Status = model.PlatformAddonStatusFailed
	addon.ErrorMessage = err.Error()
	_ = db.Save(&addon).Error
	return addon, err
}

func platformAddonCheckSpecFromConfig(raw string) (k8s.PlatformAddonCheckSpec, error) {
	var spec k8s.PlatformAddonCheckSpec
	if strings.TrimSpace(raw) == "" {
		return spec, nil
	}
	if err := json.Unmarshal([]byte(raw), &spec); err != nil {
		return spec, fmt.Errorf("parse platform addon checks: %w", err)
	}
	return spec, nil
}

func cleanAddonArchivePath(name string) string {
	clean := path.Clean(strings.TrimSpace(strings.TrimPrefix(name, "/")))
	if clean == "." || clean == "" || strings.HasPrefix(clean, "../") || clean == ".." {
		return ""
	}
	return clean
}

func defaultPlatformAddonS3Key(name string, source string) string {
	name = strings.TrimSpace(name)
	if source == model.PlatformAddonSourceCustom {
		return path.Join("platform-addons/custom", name+".tar.gz")
	}
	return path.Join("platform-addons", name+".tar.gz")
}

func compactStringList(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}
