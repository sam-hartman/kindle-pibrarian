package modes

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	"github.com/sam-hartman/kindle-pibrarian/internal/logger"
	"go.uber.org/zap"
)

type Env struct {
	SecretKey    string `json:"secret"`
	DownloadPath string `json:"download_path"`
	// Email settings for Kindle
	SMTPHost     string `json:"smtp_host"`
	SMTPPort     string `json:"smtp_port"`
	SMTPUser     string `json:"smtp_user"`
	SMTPPassword string `json:"smtp_password"`
	FromEmail    string `json:"from_email"`
	KindleEmail  string `json:"kindle_email"`
}

// loadEnvFile loads environment variables from a .env file
// It reads key=value pairs and sets them in the environment if not already set
func loadEnvFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		
		// Parse key=value
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		
		// Remove quotes if present
		if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'')) {
			value = value[1 : len(value)-1]
		}
		
		// Only set if not already in environment
		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}
	
	return scanner.Err()
}

func GetEnv() (*Env, error) {
	l := logger.GetLogger()

	// Try to load .env file from current directory or executable directory
	envPaths := []string{
		".env",
		filepath.Join(filepath.Dir(os.Args[0]), ".env"),
	}
	
	for _, envPath := range envPaths {
		if _, err := os.Stat(envPath); err == nil {
			if err := loadEnvFile(envPath); err != nil {
				l.Warn("Failed to load .env file", zap.String("path", envPath), zap.Error(err))
			} else {
				l.Info("Loaded environment variables from .env file", zap.String("path", envPath))
				break
			}
		}
	}

	secretKey := os.Getenv("ANNAS_SECRET_KEY")
	downloadPath := os.Getenv("ANNAS_DOWNLOAD_PATH")
	
	// Email settings (optional - only needed if emailing to Kindle)
	smtpHost := os.Getenv("SMTP_HOST")
	smtpPort := os.Getenv("SMTP_PORT")
	if smtpPort == "" {
		smtpPort = "587" // Default to TLS port
	}
	smtpUser := os.Getenv("SMTP_USER")
	smtpPassword := os.Getenv("SMTP_PASSWORD")
	fromEmail := os.Getenv("FROM_EMAIL")
	kindleEmail := os.Getenv("KINDLE_EMAIL")

	// SecretKey is only required for actual downloads, not for email testing
	if secretKey == "" {
		l.Warn("ANNAS_SECRET_KEY not set - downloads from Anna's Archive will fail")
	}

	// DownloadPath is optional if we're emailing instead
	if downloadPath == "" {
		downloadPath = os.TempDir() // Use temp dir if not specified
	}

	return &Env{
		SecretKey:    secretKey,
		DownloadPath: downloadPath,
		SMTPHost:     smtpHost,
		SMTPPort:     smtpPort,
		SMTPUser:     smtpUser,
		SMTPPassword: smtpPassword,
		FromEmail:    fromEmail,
		KindleEmail:  kindleEmail,
	}, nil
}
