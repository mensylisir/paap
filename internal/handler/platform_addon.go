package handler

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"strings"

	"paap/internal/database"
	"paap/internal/model"
	"paap/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const platformAddonPrefix = "platform-addons"

type PlatformAddonSyncResult struct {
	Updated int `json:"updated"`
}

func ListPlatformAddons(c *gin.Context) {
	if _, err := SyncPlatformAddonsNow(c.Request.Context()); err != nil {
		log.Printf("[ListPlatformAddons] sync skipped: %v", err)
	}
	addons, err := service.ListPlatformAddons(database.DB)
	if err != nil {
		respondPlatformAddonError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": addons})
}

func GetPlatformAddon(c *gin.Context) {
	if _, err := SyncPlatformAddonsNow(c.Request.Context()); err != nil {
		log.Printf("[GetPlatformAddon] sync skipped: %v", err)
	}
	addon, err := service.GetPlatformAddon(database.DB, c.Param("name"))
	if err != nil {
		respondPlatformAddonError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": addon})
}

func EnablePlatformAddon(c *gin.Context) {
	addon, err := service.GetPlatformAddon(database.DB, c.Param("name"))
	if err != nil {
		respondPlatformAddonError(c, err)
		return
	}
	archivePath, cleanup, err := downloadPlatformAddonPackage(c.Request.Context(), addon)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	addon, err = service.EnablePlatformAddonFromArchive(c.Request.Context(), database.DB, addon.Name, archivePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "data": addon})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": addon})
}

func DisablePlatformAddon(c *gin.Context) {
	addon, err := service.GetPlatformAddon(database.DB, c.Param("name"))
	if err != nil {
		respondPlatformAddonError(c, err)
		return
	}
	archivePath, cleanup, err := downloadPlatformAddonPackage(c.Request.Context(), addon)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	addon, err = service.DisablePlatformAddonFromArchive(c.Request.Context(), database.DB, addon.Name, archivePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "data": addon})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": addon})
}

func CheckPlatformAddon(c *gin.Context) {
	addon, err := service.CheckAndSavePlatformAddon(c.Request.Context(), database.DB, c.Param("name"))
	if err != nil {
		respondPlatformAddonError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": addon})
}

func UploadPlatformAddon(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing 'file' field"})
		return
	}
	defer file.Close()
	if !strings.HasSuffix(header.Filename, ".tar.gz") && !strings.HasSuffix(header.Filename, ".tgz") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file must be .tar.gz or .tgz"})
		return
	}

	tmp, err := os.CreateTemp("", "paap-platform-addon-upload-*.tar.gz")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server error"})
		return
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	if _, err := io.Copy(tmp, file); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read uploaded file"})
		return
	}
	if _, err := tmp.Seek(0, 0); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server error"})
		return
	}
	pkg, err := service.ParsePlatformAddonArchive(tmp.Name())
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid platform addon package: %v", err)})
		return
	}

	objectName := path.Join(platformAddonPrefix, "custom", service.SlugifyComponentConfigTemplateKey(pkg.Spec.Name)+".tar.gz")
	s3, err := getOrCreateS3Client()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("S3 client error: %v", err)})
		return
	}
	if err := s3.UploadFile(c.Request.Context(), objectName, tmp.Name(), "application/gzip"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("S3 upload error: %v", err)})
		return
	}
	pkg.Spec.S3Bucket = s3BucketName
	pkg.Spec.S3Key = objectName
	addon, err := service.UpsertPlatformAddonPackage(database.DB, pkg, model.PlatformAddonSourceCustom)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("db error: %v", err)})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": addon})
}

func SyncPlatformAddons(c *gin.Context) {
	result, err := SyncPlatformAddonsNow(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "updated": result.Updated})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "sync completed", "updated": result.Updated})
}

func SyncPlatformAddonsNow(ctx context.Context) (PlatformAddonSyncResult, error) {
	result := PlatformAddonSyncResult{}
	if database.DB == nil {
		return result, fmt.Errorf("database is not initialized")
	}
	s3, err := getOrCreateS3Client()
	if err != nil {
		return result, err
	}
	objects, err := s3.ListObjects(ctx, platformAddonPrefix+"/")
	if err != nil {
		return result, err
	}
	for _, objectName := range objects {
		if !strings.HasSuffix(objectName, ".tar.gz") && !strings.HasSuffix(objectName, ".tgz") {
			continue
		}
		tmp, err := os.CreateTemp("", "paap-platform-addon-sync-*.tar.gz")
		if err != nil {
			return result, err
		}
		tmpPath := tmp.Name()
		_ = tmp.Close()
		if err := s3.DownloadFile(ctx, objectName, tmpPath); err != nil {
			_ = os.Remove(tmpPath)
			return result, err
		}
		pkg, err := service.ParsePlatformAddonArchive(tmpPath)
		_ = os.Remove(tmpPath)
		if err != nil {
			return result, err
		}
		pkg.Spec.S3Bucket = s3BucketName
		pkg.Spec.S3Key = objectName
		source := model.PlatformAddonSourceBuiltin
		if strings.HasPrefix(objectName, platformAddonPrefix+"/custom/") {
			source = model.PlatformAddonSourceCustom
		}
		if _, err := service.UpsertPlatformAddonPackage(database.DB, pkg, source); err != nil {
			return result, err
		}
		result.Updated++
	}
	return result, nil
}

func downloadPlatformAddonPackage(ctx context.Context, addon model.ClusterAddon) (string, func(), error) {
	if strings.TrimSpace(addon.S3Key) == "" {
		return "", nil, fmt.Errorf("platform addon %s has no MinIO object key", addon.Name)
	}
	return downloadS3ObjectToTemp(ctx, addon.S3Key, "paap-platform-addon-enable-*.tar.gz")
}

func downloadS3ObjectToTemp(ctx context.Context, objectName string, pattern string) (string, func(), error) {
	s3, err := getOrCreateS3Client()
	if err != nil {
		return "", nil, err
	}
	tmp, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", nil, err
	}
	tmpPath := tmp.Name()
	_ = tmp.Close()
	if err := s3.DownloadFile(ctx, objectName, tmpPath); err != nil {
		_ = os.Remove(tmpPath)
		return "", nil, err
	}
	return tmpPath, func() { _ = os.Remove(tmpPath) }, nil
}

func respondPlatformAddonError(c *gin.Context, err error) {
	if err == gorm.ErrRecordNotFound {
		c.JSON(http.StatusNotFound, gin.H{"error": "platform addon not found"})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
}
