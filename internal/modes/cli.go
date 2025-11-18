package modes

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"net/smtp"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/fang"
	"github.com/iosifache/annas-mcp/internal/anna"
	"github.com/iosifache/annas-mcp/internal/logger"
	"github.com/iosifache/annas-mcp/internal/version"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

// sendTestFileToKindle sends a test file to Kindle to verify email configuration
func sendTestFileToKindle(fileData []byte, filename string, book *anna.Book, env *Env) error {
	l := logger.GetLogger()

	// Determine MIME type based on format
	mimeType := "application/pdf"
	if strings.HasSuffix(filename, ".epub") {
		mimeType = "application/epub+zip"
	}

	// Create email message
	var emailBody bytes.Buffer
	emailBody.WriteString(fmt.Sprintf("From: %s\r\n", env.FromEmail))
	emailBody.WriteString(fmt.Sprintf("To: %s\r\n", env.KindleEmail))
	emailBody.WriteString(fmt.Sprintf("Subject: %s\r\n", filename))
	emailBody.WriteString("MIME-Version: 1.0\r\n")
	emailBody.WriteString(fmt.Sprintf("Content-Type: multipart/mixed; boundary=boundary123\r\n\r\n"))
	
	// Email body
	emailBody.WriteString("--boundary123\r\n")
	emailBody.WriteString("Content-Type: text/plain; charset=utf-8\r\n\r\n")
	emailBody.WriteString(fmt.Sprintf("Test Book: %s\r\n", book.Title))
	emailBody.WriteString("This is a test email to verify Kindle email functionality.\r\n")
	emailBody.WriteString("\r\n")
	
	// Attachment
	emailBody.WriteString("--boundary123\r\n")
	emailBody.WriteString(fmt.Sprintf("Content-Type: %s; name=\"%s\"\r\n", mimeType, filename))
	emailBody.WriteString("Content-Transfer-Encoding: base64\r\n")
	emailBody.WriteString(fmt.Sprintf("Content-Disposition: attachment; filename=\"%s\"\r\n\r\n", filename))
	
	// Encode file as base64
	encoded := base64.StdEncoding.EncodeToString(fileData)
	// Split into lines of 76 characters (RFC 2045)
	for i := 0; i < len(encoded); i += 76 {
		end := i + 76
		if end > len(encoded) {
			end = len(encoded)
		}
		emailBody.WriteString(encoded[i:end] + "\r\n")
	}
	
	emailBody.WriteString("\r\n--boundary123--\r\n")

	// Send email via SMTP
	addr := fmt.Sprintf("%s:%s", env.SMTPHost, env.SMTPPort)
	auth := smtp.PlainAuth("", env.SMTPUser, env.SMTPPassword, env.SMTPHost)

	l.Info("Sending test file to Kindle",
		zap.String("filename", filename),
		zap.String("kindle_email", env.KindleEmail),
	)

	err := smtp.SendMail(addr, auth, env.FromEmail, []string{env.KindleEmail}, emailBody.Bytes())
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}

func StartCLI() {
	l := logger.GetLogger()
	defer l.Sync()

	rootCmd := &cobra.Command{
		Use:   "annas-mcp",
		Short: "Anna's Archive MCP CLI",
		Long:  "A command-line interface for searching and downloading books from Anna's Archive.",
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
		Version: version.GetVersion(),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	rootCmd.SetVersionTemplate("{{.Version}}\n")

	searchCmd := &cobra.Command{
		Use:   "search [term]",
		Short: "Search for books",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			searchTerm := args[0]
			l.Info("Search command called", zap.String("searchTerm", searchTerm))

			books, err := anna.FindBook(searchTerm)
			if err != nil {
				l.Error("Search command failed",
					zap.String("searchTerm", searchTerm),
					zap.Error(err),
				)
				return fmt.Errorf("failed to search books: %w", err)
			}

			if len(books) == 0 {
				fmt.Println("No books found.")
				return nil
			}

			for i, book := range books {
				fmt.Printf("Book %d:\n%s\n", i+1, book.String())
				if i < len(books)-1 {
					fmt.Println()
				}
			}

			l.Info("Search command completed successfully",
				zap.String("searchTerm", searchTerm),
				zap.Int("resultsCount", len(books)),
			)

			return nil
		},
	}

	downloadCmd := &cobra.Command{
		Use:   "download [hash] [filename]",
		Short: "Download a book by its MD5 hash",
		Long:  "Download a book by its MD5 hash to the specified filename. Requires ANNAS_SECRET_KEY and ANNAS_DOWNLOAD_PATH environment variables.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			bookHash := args[0]
			filename := args[1]

			ext := filepath.Ext(filename)
			if ext == "" {
				return fmt.Errorf("filename must include an extension (e.g., .pdf, .epub)")
			}
			format := strings.TrimPrefix(ext, ".")
			title := strings.TrimSuffix(filepath.Base(filename), ext)

			l.Info("Download command called",
				zap.String("bookHash", bookHash),
				zap.String("filename", filename),
				zap.String("title", title),
				zap.String("format", format),
			)

			env, err := GetEnv()
			if err != nil {
				l.Error("Failed to get environment variables", zap.Error(err))
				return fmt.Errorf("failed to get environment: %w", err)
			}

			book := &anna.Book{
				Hash:   bookHash,
				Title:  title,
				Format: format,
			}

			err = book.Download(env.SecretKey, env.DownloadPath)
			if err != nil {
				l.Error("Download command failed",
					zap.String("bookHash", bookHash),
					zap.String("downloadPath", env.DownloadPath),
					zap.Error(err),
				)
				return fmt.Errorf("failed to download book: %w", err)
			}

			fullPath := filepath.Join(env.DownloadPath, filename)
			fmt.Printf("Book downloaded successfully to: %s\n", fullPath)

			l.Info("Download command completed successfully",
				zap.String("bookHash", bookHash),
				zap.String("downloadPath", env.DownloadPath),
				zap.String("filename", filename),
			)

			return nil
		},
	}

	mcpCmd := &cobra.Command{
		Use:   "mcp",
		Short: "Start the MCP server",
		Long:  "Start the Model Context Protocol (MCP) server for integration with AI assistants.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Exit CLI mode and start MCP server
			StartMCPServer()
			return nil
		},
	}

	httpCmd := &cobra.Command{
		Use:   "http",
		Short: "Start the MCP HTTP server",
		Long:  "Start the MCP server as an HTTP server for integration with HTTP-based MCP clients like Mistral Le Chat.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			port, _ := cmd.Flags().GetString("port")
			StartMCPHTTPServer(port)
			return nil
		},
	}
	httpCmd.Flags().StringP("port", "p", "8080", "Port to listen on")

	testEmailCmd := &cobra.Command{
		Use:   "test-email",
		Short: "Test email functionality by sending a sample file to Kindle",
		Long:  "Creates a small test file and sends it to your Kindle email to verify email configuration.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			env, err := GetEnv()
			if err != nil {
				return fmt.Errorf("failed to get environment: %w", err)
			}

			// Check if email is configured
			if env.SMTPHost == "" || env.SMTPUser == "" || env.SMTPPassword == "" || env.FromEmail == "" {
				return fmt.Errorf("email configuration incomplete. Please set SMTP_HOST, SMTP_USER, SMTP_PASSWORD, and FROM_EMAIL")
			}

			l.Info("Testing email functionality",
				zap.String("from", env.FromEmail),
				zap.String("to", env.KindleEmail),
				zap.String("smtp", env.SMTPHost+":"+env.SMTPPort),
			)

			// Create a simple test file (small PDF content)
			testContent := []byte("%PDF-1.4\n1 0 obj\n<<\n/Type /Catalog\n>>\nendobj\nxref\n0 1\ntrailer\n<<\n/Size 1\n>>\nstartxref\n9\n%%EOF")
			filename := "test-book.pdf"

			// Create a temporary book object for emailing
			book := &anna.Book{
				Title:  "Test Book - Email Functionality",
				Format: "pdf",
			}

			// Use a helper function to send the test file
			err = sendTestFileToKindle(testContent, filename, book, env)
			if err != nil {
				return fmt.Errorf("failed to send test email: %w", err)
			}

			fmt.Printf("✅ Test email sent successfully!\n")
			fmt.Printf("   From: %s\n", env.FromEmail)
			fmt.Printf("   To: %s\n", env.KindleEmail)
			fmt.Printf("   Check your Kindle or Kindle app in a few minutes.\n")
			return nil
		},
	}

	rootCmd.AddCommand(searchCmd)
	rootCmd.AddCommand(downloadCmd)
	rootCmd.AddCommand(mcpCmd)
	rootCmd.AddCommand(httpCmd)
	rootCmd.AddCommand(testEmailCmd)

	if err := fang.Execute(
		context.Background(),
		rootCmd,
		fang.WithVersion(version.GetVersion()),
	); err != nil {
		os.Exit(1)
	}
}
