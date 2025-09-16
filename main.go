package main

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var (
	version = "1.0.0"
	commit  = "dev"
	date    = "unknown"
	db      *sql.DB
)

type ConnectParams struct {
	DSN string `json:"dsn"`
}

type ListTablesParams struct {
	Database string `json:"database"`
}

type DescribeTableParams struct {
	Database string `json:"database"`
	Table    string `json:"table"`
}

type ExecuteQueryParams struct {
	Query string `json:"query"`
}

type DatabaseInfo struct {
	Name string `json:"name"`
}

type TableInfo struct {
	TableName   string `json:"table_name"`
	TableType   string `json:"table_type"`
	TableSchema string `json:"table_schema"`
}

type ColumnInfo struct {
	ColumnName    string  `json:"column_name"`
	DataType      string  `json:"data_type"`
	IsNullable    string  `json:"is_nullable"`
	ColumnDefault *string `json:"column_default"`
	Extra         string  `json:"extra"`
}

func Connect(ctx context.Context, req *mcp.CallToolRequest, args ConnectParams) (*mcp.CallToolResult, any, error) {
	database, err := sql.Open("mysql", args.DSN)
	if err != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Failed to open database: %v", err)},
			},
		}, nil, nil
	}

	if err := database.Ping(); err != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Failed to ping database: %v", err)},
			},
		}, nil, nil
	}

	db = database
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: "Successfully connected to MySQL database"},
		},
	}, nil, nil
}

func ListDatabases(ctx context.Context, req *mcp.CallToolRequest, args struct{}) (*mcp.CallToolResult, any, error) {
	if db == nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: "Not connected to database. Use connect tool first."},
			},
		}, nil, nil
	}

	rows, err := db.QueryContext(ctx, "SHOW DATABASES")
	if err != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Failed to query databases: %v", err)},
			},
		}, nil, nil
	}
	defer rows.Close()

	var databases []DatabaseInfo
	for rows.Next() {
		var dbName string
		if err := rows.Scan(&dbName); err != nil {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{
					&mcp.TextContent{Text: fmt.Sprintf("Failed to scan database name: %v", err)},
				},
			}, nil, nil
		}
		databases = append(databases, DatabaseInfo{Name: dbName})
	}

	if err := rows.Err(); err != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Row iteration error: %v", err)},
			},
		}, nil, nil
	}

	result := fmt.Sprintf("Found %d databases:\n", len(databases))
	for _, dbInfo := range databases {
		result += fmt.Sprintf("- %s\n", dbInfo.Name)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: result},
		},
	}, databases, nil
}

func ListTables(ctx context.Context, req *mcp.CallToolRequest, args ListTablesParams) (*mcp.CallToolResult, any, error) {
	if db == nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: "Not connected to database. Use connect tool first."},
			},
		}, nil, nil
	}

	query := `
		SELECT TABLE_NAME, TABLE_TYPE, TABLE_SCHEMA
		FROM information_schema.TABLES
		WHERE TABLE_SCHEMA = ?
	`
	rows, err := db.QueryContext(ctx, query, args.Database)
	if err != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Failed to query tables: %v", err)},
			},
		}, nil, nil
	}
	defer rows.Close()

	var tables []TableInfo
	for rows.Next() {
		var table TableInfo
		if err := rows.Scan(&table.TableName, &table.TableType, &table.TableSchema); err != nil {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{
					&mcp.TextContent{Text: fmt.Sprintf("Failed to scan table info: %v", err)},
				},
			}, nil, nil
		}
		tables = append(tables, table)
	}

	if err := rows.Err(); err != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Row iteration error: %v", err)},
			},
		}, nil, nil
	}

	result := fmt.Sprintf("Found %d tables in database '%s':\n", len(tables), args.Database)
	for _, table := range tables {
		result += fmt.Sprintf("- %s (%s)\n", table.TableName, table.TableType)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: result},
		},
	}, tables, nil
}

func DescribeTable(ctx context.Context, req *mcp.CallToolRequest, args DescribeTableParams) (*mcp.CallToolResult, any, error) {
	if db == nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: "Not connected to database. Use connect tool first."},
			},
		}, nil, nil
	}

	query := `
		SELECT COLUMN_NAME, DATA_TYPE, IS_NULLABLE, COLUMN_DEFAULT, EXTRA
		FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?
		ORDER BY ORDINAL_POSITION
	`
	rows, err := db.QueryContext(ctx, query, args.Database, args.Table)
	if err != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Failed to query table columns: %v", err)},
			},
		}, nil, nil
	}
	defer rows.Close()

	var columns []ColumnInfo
	for rows.Next() {
		var col ColumnInfo
		if err := rows.Scan(&col.ColumnName, &col.DataType, &col.IsNullable, &col.ColumnDefault, &col.Extra); err != nil {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{
					&mcp.TextContent{Text: fmt.Sprintf("Failed to scan column info: %v", err)},
				},
			}, nil, nil
		}
		columns = append(columns, col)
	}

	if err := rows.Err(); err != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Row iteration error: %v", err)},
			},
		}, nil, nil
	}

	result := fmt.Sprintf("Table '%s.%s' has %d columns:\n\n", args.Database, args.Table, len(columns))
	result += fmt.Sprintf("%-20s %-15s %-10s %-15s %s\n", "Column", "Type", "Nullable", "Default", "Extra")
	result += strings.Repeat("-", 80) + "\n"

	for _, col := range columns {
		defaultVal := "NULL"
		if col.ColumnDefault != nil {
			defaultVal = *col.ColumnDefault
		}
		result += fmt.Sprintf("%-20s %-15s %-10s %-15s %s\n",
			col.ColumnName, col.DataType, col.IsNullable, defaultVal, col.Extra)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: result},
		},
	}, columns, nil
}

func ExecuteQuery(ctx context.Context, req *mcp.CallToolRequest, args ExecuteQueryParams) (*mcp.CallToolResult, any, error) {
	if db == nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: "Not connected to database. Use connect tool first."},
			},
		}, nil, nil
	}

	query := strings.TrimSpace(args.Query)
	if query == "" {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: "Query cannot be empty"},
			},
		}, nil, nil
	}

	upperQuery := strings.ToUpper(query)
	isSelect := strings.HasPrefix(upperQuery, "SELECT") ||
		strings.HasPrefix(upperQuery, "SHOW") ||
		strings.HasPrefix(upperQuery, "DESCRIBE") ||
		strings.HasPrefix(upperQuery, "EXPLAIN")

	if isSelect {
		return executeSelectQuery(ctx, query)
	} else {
		return executeModifyQuery(ctx, query)
	}
}

func executeSelectQuery(ctx context.Context, query string) (*mcp.CallToolResult, any, error) {
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Failed to execute query: %v", err)},
			},
		}, nil, nil
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Failed to get columns: %v", err)},
			},
		}, nil, nil
	}

	var results []map[string]any
	for rows.Next() {
		values := make([]any, len(columns))
		valuePtrs := make([]any, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{
					&mcp.TextContent{Text: fmt.Sprintf("Failed to scan row: %v", err)},
				},
			}, nil, nil
		}

		row := make(map[string]any)
		for i, col := range columns {
			val := values[i]
			if b, ok := val.([]byte); ok {
				row[col] = string(b)
			} else {
				row[col] = val
			}
		}
		results = append(results, row)
	}

	if err := rows.Err(); err != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Row iteration error: %v", err)},
			},
		}, nil, nil
	}

	resultText := fmt.Sprintf("Query executed successfully. Returned %d rows:\n\n", len(results))

	if len(results) > 0 {
		for i, col := range columns {
			resultText += fmt.Sprintf("%-20s", col)
			if i < len(columns)-1 {
				resultText += " | "
			}
		}
		resultText += "\n" + strings.Repeat("-", len(columns)*23) + "\n"

		for _, row := range results {
			for i, col := range columns {
				val := row[col]
				if val == nil {
					val = "NULL"
				}
				resultText += fmt.Sprintf("%-20v", val)
				if i < len(columns)-1 {
					resultText += " | "
				}
			}
			resultText += "\n"
		}
	}

	return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: resultText},
			},
		}, map[string]any{
			"rows":     results,
			"rowCount": len(results),
			"columns":  columns,
		}, nil
}

func executeModifyQuery(ctx context.Context, query string) (*mcp.CallToolResult, any, error) {
	result, err := db.ExecContext(ctx, query)
	if err != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Failed to execute query: %v", err)},
			},
		}, nil, nil
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Failed to get rows affected: %v", err)},
			},
		}, nil, nil
	}

	lastInsertId, err := result.LastInsertId()
	if err != nil {
		lastInsertId = -1
	}

	resultText := fmt.Sprintf("Query executed successfully.\nRows affected: %d", rowsAffected)
	if lastInsertId != -1 {
		resultText += fmt.Sprintf("\nLast insert ID: %d", lastInsertId)
	}

	return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: resultText},
			},
		}, map[string]any{
			"rowsAffected": rowsAffected,
			"lastInsertId": lastInsertId,
		}, nil
}

type GitHubRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

func updateSelf() error {
	fmt.Println("Checking for updates...")

	// Get latest release from GitHub API
	resp, err := http.Get("https://api.github.com/repos/josiah-hester/mysql-mcp/releases/latest")
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to get release info: HTTP %d", resp.StatusCode)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return fmt.Errorf("failed to parse release info: %w", err)
	}

	if release.TagName == "v"+version {
		fmt.Printf("Already up to date (version %s)\n", version)
		return nil
	}

	fmt.Printf("Found newer version: %s (current: %s)\n", release.TagName, version)

	// Find the correct asset for current OS and architecture
	osName := runtime.GOOS
	if osName == "darwin" {
		osName = "Darwin"
	} else if osName == "linux" {
		osName = "Linux"
	} else if osName == "windows" {
		osName = "Windows"
	}

	archName := runtime.GOARCH
	if archName == "amd64" {
		archName = "x86_64"
	}

	var downloadURL string
	expectedName := fmt.Sprintf("mysql-mcp_%s_%s", osName, archName)

	for _, asset := range release.Assets {
		if strings.Contains(asset.Name, expectedName) {
			downloadURL = asset.BrowserDownloadURL
			break
		}
	}

	if downloadURL == "" {
		return fmt.Errorf("no compatible release found for %s %s", osName, archName)
	}

	fmt.Printf("Downloading %s...\n", downloadURL)

	// Download the release
	resp, err = http.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("failed to download update: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download update: HTTP %d", resp.StatusCode)
	}

	// Get current executable path
	currentExe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get current executable path: %w", err)
	}

	// Create temporary file for the new binary
	tempFile := currentExe + ".new"

	// Extract and save the new binary
	if err := extractBinary(resp.Body, tempFile); err != nil {
		return fmt.Errorf("failed to extract update: %w", err)
	}

	// Make the new binary executable
	if err := os.Chmod(tempFile, 0755); err != nil {
		os.Remove(tempFile)
		return fmt.Errorf("failed to make new binary executable: %w", err)
	}

	// Replace the current binary
	if err := os.Rename(tempFile, currentExe); err != nil {
		os.Remove(tempFile)
		return fmt.Errorf("failed to replace binary: %w", err)
	}

	fmt.Printf("Successfully updated to version %s\n", release.TagName)
	fmt.Println("Please restart the application to use the new version.")

	return nil
}

func extractBinary(src io.Reader, destPath string) error {
	// Create destination file
	destFile, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer destFile.Close()

	// Check if it's a gzipped tar archive
	gzReader, err := gzip.NewReader(src)
	if err != nil {
		// If it's not gzipped, assume it's a raw binary and copy directly
		_, copyErr := io.Copy(destFile, src)
		return copyErr
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)

	// Find the binary file in the tar archive
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// Look for the binary (usually the file without extension or with .exe)
		filename := filepath.Base(header.Name)
		if strings.HasPrefix(filename, "mysql-mcp") && header.Typeflag == tar.TypeReg {
			// Copy the binary content
			_, err := io.Copy(destFile, tarReader)
			return err
		}
	}

	return fmt.Errorf("binary not found in archive")
}

func main() {
	dsn := flag.String("dsn", "", "MySQL DSN (e.g., user:password@tcp(localhost:3306)/database)")
	versionFlag := flag.Bool("version", false, "Print version information")
	updateFlag := flag.Bool("update", false, "Update to the latest version from GitHub")
	flag.Parse()

	if *versionFlag {
		fmt.Printf("mysql-mcp-server version %s\n", version)
		fmt.Printf("Commit: %s\n", commit)
		fmt.Printf("Built: %s\n", date)
		return
	}

	if *updateFlag {
		if err := updateSelf(); err != nil {
			log.Fatalf("Update failed: %v", err)
		}
		return
	}

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "mysql-mcp-server",
		Version: "1.0.0",
	}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "connect",
		Description: "Connect to MySQL database using DSN (e.g., user:password@tcp(localhost:3306)/)",
	}, Connect)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_databases",
		Description: "List all databases on the MySQL server",
	}, ListDatabases)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_tables",
		Description: "List all tables in a specific database",
	}, ListTables)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "describe_table",
		Description: "Describe the structure of a specific table",
	}, DescribeTable)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "execute_query",
		Description: "Execute a SQL query (SELECT queries return data, other queries return affected row count)",
	}, ExecuteQuery)

	// Auto-connect if DSN is provided
	if *dsn != "" {
		database, err := sql.Open("mysql", *dsn)
		if err != nil {
			log.Fatalf("Failed to open database: %v", err)
		}

		if err := database.Ping(); err != nil {
			log.Fatalf("Failed to ping database: %v", err)
		}

		db = database
		log.Printf("Successfully connected to MySQL database with DSN: %s", *dsn)
	}

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Printf("Server failed: %v", err)
	}
}
