# AGENTS.md

This document provides essential guidelines and instructions for working in the `sitemap-checker` repository. This guide is intended for agents contributing to the repository to ensure consistency in development practices, testing, and coding standards.

---

## **Build, Lint, and Test Commands**

### **Build Process**
To build the project, ensure you have Go installed on your system. Run the following command in the root of the repository:

```bash
go build -o sitemap-checker
```
This will generate the `sitemap-checker` executable in the project directory.

### **Running the Application**
#### Single Sitemap Validation:
```bash
./sitemap-checker -uri=http://sitename.com/sitemap.xml -out=output.xml
```
#### Sitemap Index File Validation:
```bash
./sitemap-checker -uri=http://sitename.com/sitemap.xml -index
```

### **Testing**
Unit tests are essential to maintain code quality. To run all tests, use:

```bash
go test ./...
```
To run a specific test file:
```bash
go test -v ./path/to/testfile.go
```
To run a single test function:
```bash
go test -run ^TestFunctionName$
```

### **Linting**
To ensure code adheres to Go’s standards, install and run 
`golangci-lint`:
```bash
golangci-lint run
```
If not installed, you can get it via:
```bash
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

---

## **Code Styling Guidelines**
Maintaining consistent code style is critical. Follow these guidelines:

### **Formatting**
- Use Go’s built-in formatting tool:

```bash
go fmt ./...
```
- Always format code before committing.

### **Imports**
- Group imports into three sections:
    1. Standard library packages
    2. Third-party packages
    3. Internal packages
- Use `goimports` to manage imports automatically:

```bash
go install golang.org/x/tools/cmd/goimports@latest
goimports -w .
```

### **Types and Variables**
- Use clear and descriptive names.
- Prefer short names for local variables and longer names for package-level declarations.
- Use `camelCase` for variable names and `PascalCase` for exported types.
- Avoid global variables unless necessary.

### **Error Handling**
- Always check and handle errors. Do not ignore returned errors.
- Use the `errors` or `fmt.Errorf` packages to create new errors:

```go
import "errors"
if err != nil {
    return errors.New("specific error message")
}
```
- Wrap errors to provide more context:

```go
fmt.Errorf("failed to read file: %w", err)
```

### **Logging**
- Use `log` for logging messages.
- Use proper log levels (`Info`, `Debug`, `Error`, etc.) based on the situation.

```go
import "log"
log.Println("This is a message")
```

### **Naming Conventions**
- Function names should be descriptive and start with a verb (e.g., `validateSitemap`).
- File names should use underscores (snake_case).
- Test functions should begin with the word `Test` (e.g., `TestValidateSitemap`).

### **Comments**
- Use comments for package declarations, exported functions, and complex logic.
- Follow GoDoc conventions:
```go
// validateSitemap checks the validity of a sitemap file.
func validateSitemap(file string) error {
}
```

### **Testing**
- Test exported and critical unexported functions.
- Use table-driven tests for multiple scenarios.
- Run tests locally before pushing changes.

Example of a table-driven test:
```go
func TestMyFunction(t *testing.T) {
    tests := []struct {
        name string
        input int
        want  int
    }{
        {"test case 1", 1, 2},
        {"test case 2", 2, 4},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            if got := MyFunction(tt.input); got != tt.want {
                t.Errorf("MyFunction() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

### **Folder Structure**
Keep the file organization intuitive:
- `/cmd` for command-line utilities.
- `/pkg` for library code that can be imported by other projects.
- `/internal` for code that cannot be imported by external projects.
- `/test` for integration tests.

### **VS Code Integration**
Ensure your editor is configured for Go development. Recommended VS Code extensions:
- [Go](https://marketplace.visualstudio.com/items?itemName=golang.Go)
- Add the following settings to `.vscode/settings.json`:
```json
{
    "go.formatTool": "goimports",
    "gopls": {
        "staticcheck": true
    }
}
```

---

By adhering to these guidelines, agents can contribute to `sitemap-checker` efficiently and maintain high code quality!
