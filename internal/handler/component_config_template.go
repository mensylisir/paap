package handler

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"paap/internal/database"
	"paap/internal/model"
	"paap/internal/service"

	"github.com/gin-gonic/gin"
)

const componentConfigTemplateSyntax = service.ComponentConfigTemplateSyntax

type componentConfigTemplateRequest struct {
	Key            string                   `json:"key"`
	Name           string                   `json:"name"`
	Description    string                   `json:"description"`
	Framework      string                   `json:"framework"`
	BindingMode    string                   `json:"bindingMode"`
	ComponentTypes []string                 `json:"componentTypes"`
	S3Bucket       string                   `json:"s3Bucket"`
	S3Key          string                   `json:"s3Key"`
	Syntax         string                   `json:"syntax"`
	NativeConfigs  []map[string]interface{} `json:"nativeConfigs"`
	Fields         []map[string]interface{} `json:"fields"`
	Env            []map[string]interface{} `json:"env"`
	ConfigMaps     []map[string]interface{} `json:"configMaps"`
	Secrets        []map[string]interface{} `json:"secrets"`
	Files          []map[string]interface{} `json:"files"`
	Command        []string                 `json:"command"`
	Args           []string                 `json:"args"`
	Enabled        *bool                    `json:"enabled"`
}

type componentConfigTemplateResponse struct {
	ID             uint                     `json:"id"`
	Key            string                   `json:"key"`
	Name           string                   `json:"name"`
	Description    string                   `json:"description"`
	Framework      string                   `json:"framework"`
	BindingMode    string                   `json:"bindingMode"`
	ComponentTypes []string                 `json:"componentTypes"`
	S3Bucket       string                   `json:"s3Bucket,omitempty"`
	S3Key          string                   `json:"s3Key,omitempty"`
	Syntax         string                   `json:"syntax"`
	NativeConfigs  []map[string]interface{} `json:"nativeConfigs"`
	Fields         []map[string]interface{} `json:"fields"`
	Env            []map[string]interface{} `json:"env"`
	ConfigMaps     []map[string]interface{} `json:"configMaps"`
	Secrets        []map[string]interface{} `json:"secrets"`
	Files          []map[string]interface{} `json:"files"`
	Command        []string                 `json:"command"`
	Args           []string                 `json:"args"`
	IsBuiltin      bool                     `json:"isBuiltin"`
	SortOrder      int                      `json:"sortOrder"`
	Enabled        bool                     `json:"enabled"`
}

// ListComponentConfigTemplates returns component runtime configuration
// templates created through the product UI/API.
func ListComponentConfigTemplates(c *gin.Context) {
	templates, err := service.ListComponentConfigTemplates(database.DB)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	items := make([]componentConfigTemplateResponse, 0, len(templates))
	for _, tmpl := range templates {
		items = append(items, componentConfigTemplateToResponse(tmpl))
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

// CreateComponentConfigTemplate creates or updates a component config template.
func CreateComponentConfigTemplate(c *gin.Context) {
	var req componentConfigTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	tmpl, err := componentConfigTemplateFromRequest(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	saved, created, err := service.CreateComponentConfigTemplate(database.DB, tmpl)
	if err != nil {
		respondComponentConfigTemplateServiceError(c, err)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	c.JSON(status, gin.H{"data": componentConfigTemplateToResponse(saved)})
}

// UploadComponentConfigTemplate uploads a user-provided config template package
// to MinIO, parses it, and indexes the parsed template in the database.
func UploadComponentConfigTemplate(c *gin.Context) {
	mode := strings.ToLower(strings.TrimSpace(c.PostForm("mode")))
	if mode == "" {
		mode = "native"
	}
	uploadedFiles, err := componentConfigTemplateUploadFiles(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(uploadedFiles) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "config template file is required"})
		return
	}
	if mode == "advanced" && len(uploadedFiles) > 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "advanced template upload accepts one JSON or .tar.gz package"})
		return
	}

	temps := make([]uploadedComponentConfigTemplateFile, 0, len(uploadedFiles))
	for _, file := range uploadedFiles {
		tmpPath, err := saveComponentConfigTemplateUpload(file)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		defer os.Remove(tmpPath)
		temps = append(temps, uploadedComponentConfigTemplateFile{Name: file.Filename, Path: tmpPath})
	}

	opts := service.ComponentConfigTemplateUploadOptions{
		Key:            strings.TrimSpace(c.PostForm("key")),
		Name:           strings.TrimSpace(c.PostForm("name")),
		Description:    strings.TrimSpace(c.PostForm("description")),
		Framework:      strings.TrimSpace(c.PostForm("framework")),
		BindingMode:    strings.TrimSpace(c.PostForm("bindingMode")),
		Mode:           mode,
		FileName:       strings.TrimSpace(c.PostForm("fileName")),
		ComponentTypes: strings.Split(strings.TrimSpace(c.PostForm("componentTypes")), ","),
	}
	var tmpl model.ComponentConfigTemplate
	var sourcePath string
	var sourceFileName string
	if len(temps) == 1 {
		sourcePath = temps[0].Path
		sourceFileName = temps[0].Name
		tmpl, err = service.ParseUploadedComponentConfigTemplateFile(temps[0].Path, temps[0].Name, opts)
	} else {
		nativeFiles := make([]service.NativeComponentConfigTemplateFile, 0, len(temps))
		for _, tmp := range temps {
			data, err := os.ReadFile(tmp.Path)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("read upload %s: %v", tmp.Name, err)})
				return
			}
			nativeFiles = append(nativeFiles, service.NativeComponentConfigTemplateFile{Name: tmp.Name, Content: string(data)})
		}
		tmpl, err = service.ParseNativeComponentConfigTemplateFiles(nativeFiles, opts)
		if err == nil {
			sourcePath, err = packageComponentConfigTemplateUploads(temps)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("package native configs: %v", err)})
				return
			}
			defer os.Remove(sourcePath)
			sourceFileName = firstNonEmpty(tmpl.Key, tmpl.Name, "native-configs") + ".tar.gz"
		}
	}
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	objectName := path.Join("config-templates/custom", fmt.Sprintf("%d-%s%s", time.Now().UnixNano(), service.SlugifyComponentConfigTemplateKey(firstNonEmpty(tmpl.Key, tmpl.Name, sourceFileName)), service.UploadTemplateFileExt(sourceFileName)))
	s3, err := getOrCreateS3Client()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("S3 client error: %v", err)})
		return
	}
	if err := s3.UploadFile(c.Request.Context(), objectName, sourcePath, componentConfigTemplateUploadContentType(sourceFileName)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("S3 upload error: %v", err)})
		return
	}
	tmpl.S3Bucket = s3BucketName
	tmpl.S3Key = objectName
	tmpl.IsBuiltin = false

	saved, created, err := service.CreateComponentConfigTemplate(database.DB, tmpl)
	if err != nil {
		respondComponentConfigTemplateServiceError(c, err)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	c.JSON(status, gin.H{"data": componentConfigTemplateToResponse(saved)})
}

type uploadedComponentConfigTemplateFile struct {
	Name string
	Path string
}

func componentConfigTemplateUploadFiles(c *gin.Context) ([]*multipart.FileHeader, error) {
	form, err := c.MultipartForm()
	if err != nil {
		return nil, fmt.Errorf("read multipart form: %w", err)
	}
	files := make([]*multipart.FileHeader, 0)
	if form != nil && form.File != nil {
		files = append(files, form.File["files"]...)
		files = append(files, form.File["file"]...)
	}
	return files, nil
}

func saveComponentConfigTemplateUpload(file *multipart.FileHeader) (string, error) {
	opened, err := file.Open()
	if err != nil {
		return "", fmt.Errorf("open upload %s: %w", file.Filename, err)
	}
	defer opened.Close()
	tmp, err := os.CreateTemp("", "paap-component-config-template-*")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	defer tmp.Close()
	if _, err := io.Copy(tmp, opened); err != nil {
		os.Remove(tmp.Name())
		return "", fmt.Errorf("read upload %s: %w", file.Filename, err)
	}
	return tmp.Name(), nil
}

func packageComponentConfigTemplateUploads(files []uploadedComponentConfigTemplateFile) (string, error) {
	tmp, err := os.CreateTemp("", "paap-component-config-template-native-*.tar.gz")
	if err != nil {
		return "", err
	}
	gz := gzip.NewWriter(tmp)
	tw := tar.NewWriter(gz)
	seen := map[string]int{}
	for _, file := range files {
		data, err := os.ReadFile(file.Path)
		if err != nil {
			tw.Close()
			gz.Close()
			tmp.Close()
			os.Remove(tmp.Name())
			return "", err
		}
		name := cleanComponentConfigTemplateUploadName(file.Name)
		if name == "" {
			name = "config"
		}
		if count := seen[name]; count > 0 {
			ext := path.Ext(name)
			name = strings.TrimSuffix(name, ext) + fmt.Sprintf("-%d%s", count+1, ext)
		}
		seen[name]++
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0644, Size: int64(len(data))}); err != nil {
			tw.Close()
			gz.Close()
			tmp.Close()
			os.Remove(tmp.Name())
			return "", err
		}
		if _, err := tw.Write(data); err != nil {
			tw.Close()
			gz.Close()
			tmp.Close()
			os.Remove(tmp.Name())
			return "", err
		}
	}
	if err := tw.Close(); err != nil {
		gz.Close()
		tmp.Close()
		os.Remove(tmp.Name())
		return "", err
	}
	if err := gz.Close(); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return "", err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return "", err
	}
	return tmp.Name(), nil
}

func cleanComponentConfigTemplateUploadName(fileName string) string {
	name := path.Clean(strings.TrimSpace(strings.ReplaceAll(fileName, "\\", "/")))
	name = strings.TrimPrefix(name, "./")
	if name == "." || strings.HasPrefix(name, "../") || strings.HasPrefix(name, "/") {
		return path.Base(fileName)
	}
	return name
}

func componentConfigTemplateUploadContentType(fileName string) string {
	lower := strings.ToLower(strings.TrimSpace(fileName))
	switch {
	case strings.HasSuffix(lower, ".tar.gz"):
		return "application/gzip"
	case strings.HasSuffix(lower, ".json"):
		return "application/json"
	case strings.HasSuffix(lower, ".yml"), strings.HasSuffix(lower, ".yaml"):
		return "application/x-yaml"
	default:
		return "text/plain"
	}
}

func componentConfigTemplateID(c *gin.Context) (uint, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "template not found"})
		return 0, false
	}
	return uint(id), true
}

func respondComponentConfigTemplateServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrComponentConfigTemplateNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "template not found"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}

// UpdateComponentConfigTemplate updates a custom component config template.
func UpdateComponentConfigTemplate(c *gin.Context) {
	id, ok := componentConfigTemplateID(c)
	if !ok {
		return
	}
	var req componentConfigTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	tmpl, err := componentConfigTemplateFromRequest(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	saved, err := service.UpdateComponentConfigTemplate(database.DB, id, tmpl)
	if err != nil {
		respondComponentConfigTemplateServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": componentConfigTemplateToResponse(saved)})
}

// DeleteComponentConfigTemplate deletes a custom component config template.
func DeleteComponentConfigTemplate(c *gin.Context) {
	id, ok := componentConfigTemplateID(c)
	if !ok {
		return
	}
	if err := service.DeleteComponentConfigTemplate(database.DB, id); err != nil {
		respondComponentConfigTemplateServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func componentConfigTemplateFromRequest(req componentConfigTemplateRequest) (model.ComponentConfigTemplate, error) {
	return service.ComponentConfigTemplateFromInput(service.ComponentConfigTemplateInput{
		Key:            req.Key,
		Name:           req.Name,
		Description:    req.Description,
		Framework:      req.Framework,
		BindingMode:    req.BindingMode,
		ComponentTypes: req.ComponentTypes,
		S3Bucket:       req.S3Bucket,
		S3Key:          req.S3Key,
		Syntax:         req.Syntax,
		NativeConfigs:  req.NativeConfigs,
		Fields:         req.Fields,
		Env:            req.Env,
		ConfigMaps:     req.ConfigMaps,
		Secrets:        req.Secrets,
		Files:          req.Files,
		Command:        req.Command,
		Args:           req.Args,
		Enabled:        req.Enabled,
	})
}

func componentConfigTemplateToResponse(tmpl model.ComponentConfigTemplate) componentConfigTemplateResponse {
	return componentConfigTemplateResponse{
		ID:             tmpl.ID,
		Key:            tmpl.Key,
		Name:           tmpl.Name,
		Description:    tmpl.Description,
		Framework:      firstNonEmpty(tmpl.Framework, "auto"),
		BindingMode:    firstNonEmpty(tmpl.BindingMode, "recommended"),
		ComponentTypes: service.DecodeStringArray(tmpl.ComponentTypes),
		S3Bucket:       tmpl.S3Bucket,
		S3Key:          tmpl.S3Key,
		Syntax:         firstNonEmpty(tmpl.Syntax, componentConfigTemplateSyntax),
		NativeConfigs:  service.DecodeObjectArray(tmpl.NativeJSON),
		Fields:         service.DecodeObjectArray(tmpl.FieldsJSON),
		Env:            service.DecodeObjectArray(tmpl.EnvJSON),
		ConfigMaps:     service.DecodeObjectArray(tmpl.ConfigJSON),
		Secrets:        service.DecodeObjectArray(tmpl.SecretJSON),
		Files:          service.NormalizeComponentConfigTemplateFileHints(service.DecodeObjectArray(tmpl.FileJSON)),
		Command:        service.DecodeStringArray(tmpl.CommandJSON),
		Args:           service.DecodeStringArray(tmpl.ArgsJSON),
		IsBuiltin:      tmpl.IsBuiltin,
		SortOrder:      tmpl.SortOrder,
		Enabled:        tmpl.Enabled,
	}
}

func SeedBuiltinComponentConfigTemplatesToS3(ctx context.Context, force bool) (BuiltInTemplateSyncResult, error) {
	s3, err := getOrCreateS3Client()
	if err != nil {
		return BuiltInTemplateSyncResult{}, err
	}
	archives, err := service.BuiltInComponentConfigTemplateArchivePaths()
	if err != nil {
		return BuiltInTemplateSyncResult{}, err
	}
	uploaded := 0
	for _, archivePath := range archives {
		objectName := path.Join("config-templates/builtin", filepath.Base(archivePath))
		if !force && s3.ObjectExists(ctx, objectName) {
			continue
		}
		if err := s3.UploadFile(ctx, objectName, archivePath, "application/gzip"); err != nil {
			return BuiltInTemplateSyncResult{Updated: uploaded}, fmt.Errorf("upload component config template %s: %w", archivePath, err)
		}
		uploaded++
	}
	return BuiltInTemplateSyncResult{Updated: uploaded}, nil
}

func SyncComponentConfigTemplatesFromS3Now(ctx context.Context) (BuiltInTemplateSyncResult, error) {
	s3, err := getOrCreateS3Client()
	if err != nil {
		return BuiltInTemplateSyncResult{}, err
	}
	objects, err := s3.ListObjects(ctx, "config-templates/builtin/")
	if err != nil {
		return BuiltInTemplateSyncResult{}, err
	}
	updated := 0
	for _, objectName := range objects {
		if !strings.HasSuffix(objectName, ".tar.gz") {
			continue
		}
		tmp, err := os.CreateTemp("", "paap-builtin-component-config-*.tar.gz")
		if err != nil {
			return BuiltInTemplateSyncResult{Updated: updated}, err
		}
		tmpPath := tmp.Name()
		tmp.Close()
		if err := s3.DownloadFile(ctx, objectName, tmpPath); err != nil {
			os.Remove(tmpPath)
			return BuiltInTemplateSyncResult{Updated: updated}, err
		}
		tmpl, err := service.ParseComponentConfigTemplatePackageFile(tmpPath)
		os.Remove(tmpPath)
		if err != nil {
			log.Printf("[SyncComponentConfigTemplates] failed to parse %s: %v", objectName, err)
			continue
		}
		tmpl.S3Bucket = s3BucketName
		tmpl.S3Key = objectName
		tmpl.IsBuiltin = true
		if tmpl.SortOrder == 0 {
			tmpl.SortOrder = 100
		}
		_, changed, err := service.CreateComponentConfigTemplate(database.DB, tmpl)
		if err != nil {
			return BuiltInTemplateSyncResult{Updated: updated}, err
		}
		if changed {
			updated++
		}
	}
	return BuiltInTemplateSyncResult{Updated: updated}, nil
}
