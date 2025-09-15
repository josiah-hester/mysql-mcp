package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var db *sql.DB

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

	var results []map[string]interface{}
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
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

		row := make(map[string]interface{})
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
		}, map[string]interface{}{
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
		}, map[string]interface{}{
			"rowsAffected": rowsAffected,
			"lastInsertId": lastInsertId,
		}, nil
}

func main() {
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

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Printf("Server failed: %v", err)
	}
}
