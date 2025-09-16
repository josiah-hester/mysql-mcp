# MySQL MCP Server

A Model Context Protocol (MCP) server that provides tools for interacting with MySQL databases.

## Features

The server provides the following tools:

### `connect`
Connect to a MySQL database using a Data Source Name (DSN).

**Parameters:**
- `dsn` (string): MySQL connection string (e.g., `user:password@tcp(localhost:3306)/database`)

**Example:**
```json
{
  "dsn": "root:password@tcp(localhost:3306)/"
}
```

### `list_databases`
List all databases on the MySQL server.

**Parameters:** None

### `list_tables`
List all tables in a specific database.

**Parameters:**
- `database` (string): Database name

**Example:**
```json
{
  "database": "myapp"
}
```

### `describe_table`
Describe the structure of a specific table, including columns, data types, and constraints.

**Parameters:**
- `database` (string): Database name
- `table` (string): Table name

**Example:**
```json
{
  "database": "myapp",
  "table": "users"
}
```

### `execute_query`
Execute a SQL query. SELECT queries return data, while other queries return the number of affected rows.

**Parameters:**
- `query` (string): SQL query to execute

**Example:**
```json
{
  "query": "SELECT * FROM users WHERE active = 1 LIMIT 10"
}
```

## Building

```bash
go build -o mysql-mcp-server
```

## Usage

The server runs over stdin/stdout and communicates using the MCP protocol.

### Command Line Options

- `-dsn string`: MySQL DSN for automatic connection on startup (optional)

### Examples

#### Basic usage (connect manually via MCP tools):
```bash
./mysql-mcp-server
```

#### Auto-connect on startup:
```bash
./mysql-mcp-server -dsn="readonly_user:password@tcp(localhost:3306)/myapp"
```

### Example with Claude Desktop

Add to your MCP configuration:

```json
{
  "mcpServers": {
    "mysql": {
      "command": "/path/to/mysql-mcp-server",
      "args": ["-dsn=readonly_user:password@tcp(localhost:3306)/myapp"]
    }
  }
}
```

Or without auto-connect:

```json
{
  "mcpServers": {
    "mysql": {
      "command": "/path/to/mysql-mcp-server"
    }
  }
}
```

### Security Considerations

- Always use the principle of least privilege when configuring database connections
- Consider using read-only database users for query operations
- Be cautious with `execute_query` tool as it can run any SQL statement
- Never commit database credentials to version control

## Dependencies

- Go 1.24+
- [go-sql-driver/mysql](https://github.com/go-sql-driver/mysql) - MySQL driver
- [modelcontextprotocol/go-sdk](https://github.com/modelcontextprotocol/go-sdk) - MCP SDK

## Error Handling

The server includes comprehensive error handling for:
- Database connection failures
- Invalid SQL queries
- Missing parameters
- Network timeouts
- Permission errors

All errors are returned as MCP tool call results with appropriate error messages.