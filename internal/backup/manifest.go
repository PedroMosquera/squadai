package backup

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// Manifest records a backup snapshot for rollback/restore.
type Manifest struct {
	ID            string         `json:"id"`
	Timestamp     time.Time      `json:"timestamp"`
	Command       string         `json:"command"`
	AffectedFiles []FileSnapshot `json:"affected_files"`
	Status        string         `json:"status"` // "complete", "rolled_back", "restored"
}

// FileSnapshot records one file's state before mutation.
type FileSnapshot struct {
	Path           string `json:"path"`
	ChecksumBefore string `json:"checksum_before,omitempty"` // SHA-256 hex; empty if file didn't exist
	ExistedBefore  bool   `json:"existed_before"`
	BackupFile     string `json:"backup_file"` // relative path within backup dir (e.g., "files/0")
}

// GenerateID creates a unique backup ID based on timestamp + random suffix.
func GenerateID() string {
	now := time.Now().UTC()
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand: " + err.Error())
	}
	return fmt.Sprintf("%s-%s", now.Format("20060102T150405Z"), hex.EncodeToString(b))
}

// Checksum computes SHA-256 hex digest of the given data.
func Checksum(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// ChecksumFile computes SHA-256 hex digest of a file's contents.
func ChecksumFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// ResolveBackupDir expands ~ in the backup directory path and returns
// an absolute path. If backupDir is empty, returns the default location.
func ResolveBackupDir(backupDir, homeDir string) string {
	if backupDir == "" {
		return fmt.Sprintf("%s/.squadai/backups", homeDir)
	}
	if strings.HasPrefix(backupDir, "~/") {
		return homeDir + backupDir[1:]
	}
	return backupDir
}
