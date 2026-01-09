package modes

type SearchParams struct {
	SearchTerm     string `json:"term" mcp:"Term to search for"`
	PreferredFormat string `json:"format,omitempty" mcp:"Optional: Preferred format (epub, pdf, mobi). Defaults to epub for Kindle compatibility."`
}

type DownloadParams struct {
	BookHash    string `json:"hash" mcp:"MD5 hash of the book to download"`
	Title       string `json:"title" mcp:"Book title, used for filename"`
	Format      string `json:"format" mcp:"Book format, for example pdf or epub"`
	KindleEmail string `json:"kindle_email,omitempty" mcp:"Optional Kindle email to send the book to. If not specified, uses the default configured KINDLE_EMAIL."`
}
