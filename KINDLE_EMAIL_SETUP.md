# Kindle Email Setup Guide

The MCP server now supports automatically emailing downloaded books directly to your Kindle!

## Configuration

### Recommended: Use .env File

The easiest way to configure the server is using a `.env` file:

1. Copy the example file:
   ```bash
   cp .env.example .env
   ```

2. Edit `.env` and fill in your email settings:
   ```bash
   # SMTP Server (Gmail example)
   SMTP_HOST=smtp.gmail.com
   SMTP_PORT=587
   
   # Your email credentials (use an App Password for Gmail)
   SMTP_USER=your-email@gmail.com
   SMTP_PASSWORD=your-app-password
   FROM_EMAIL=your-email@gmail.com
   
   # Your Kindle email address
   KINDLE_EMAIL=your-kindle-email@kindle.com
   
   # Anna's Archive API key (required)
   ANNAS_SECRET_KEY=your-api-key
   
   # Download path (optional - used as backup)
   ANNAS_DOWNLOAD_PATH=/Users/yourusername/Downloads/Anna's Archive
   ```

**Note**: The `.env` file is gitignored and will not be committed. All sensitive values should go here.

### Alternative: Environment Variables

You can also set environment variables directly before starting the server:

```bash
# SMTP Server (Gmail example)
export SMTP_HOST="smtp.gmail.com"
export SMTP_PORT="587"

# Your email credentials (use an App Password for Gmail)
export SMTP_USER="your-email@gmail.com"
export SMTP_PASSWORD="your-app-password"

# The email address that will send to Kindle (must be whitelisted in Kindle settings)
export FROM_EMAIL="your-email@gmail.com"

# Your Kindle email
export KINDLE_EMAIL="your-kindle-email@kindle.com"

# Anna's Archive API key
export ANNAS_SECRET_KEY="your-api-key"

# Download path (optional if emailing - used as backup)
export ANNAS_DOWNLOAD_PATH="/Users/yourusername/Downloads/Anna's Archive"
```

## Gmail Setup

1. **Enable 2-Factor Authentication** on your Google account
2. **Create an App Password**:
   - Go to https://myaccount.google.com/apppasswords
   - Select "Mail" and "Other (Custom name)"
   - Name it "Anna's Archive MCP"
   - Copy the 16-character password
   - Use this as `SMTP_PASSWORD`

3. **Whitelist your email in Kindle**:
   - Go to Amazon → Manage Your Content and Devices
   - Settings → Personal Document Settings
   - Add your email address (`FROM_EMAIL`) to "Approved Personal Document E-mail List"

## Other Email Providers

### Outlook/Office 365
```bash
export SMTP_HOST="smtp.office365.com"
export SMTP_PORT="587"
```

### Yahoo
```bash
export SMTP_HOST="smtp.mail.yahoo.com"
export SMTP_PORT="587"
```

### Custom SMTP
Use your provider's SMTP settings. Most use port 587 for TLS.

## How It Works

1. When you download a book via the MCP server, it will:
   - First try to email it to your Kindle
   - If email is not configured, it falls back to saving to disk
   - The file is also saved locally as a backup

2. The email includes:
   - Book title as the subject
   - Book file as an attachment
   - Proper MIME types for Kindle compatibility

3. Kindle will automatically convert and deliver the book to your device!

## Testing

After setting up, restart your MCP server and try downloading a book. Check the logs to see if the email was sent successfully.

