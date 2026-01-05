# API Documentation

This directory contains the auto-generated Swagger/OpenAPI documentation for the Listing API.

## Generated Files

- `docs.go` - Go package with embedded documentation
- `swagger.json` - OpenAPI 3.0 specification in JSON format
- `swagger.yaml` - OpenAPI 3.0 specification in YAML format

## Accessing the Documentation

Once the server is running, you can access the interactive Swagger UI at:

```
http://localhost:8080/swagger/index.html
```

## Regenerating Documentation

If you make changes to the API endpoints or Swagger annotations, regenerate the documentation with:

```bash
swag init -g cmd/listing/main.go -o docs --parseDependency --parseInternal
```

## Swagger Annotations

The API documentation is generated from special comments in the code:

**General API Info**: `cmd/listing/main.go`
**Endpoint Definitions**: 
- `internal/listing/handler/handler.go`
- `internal/authnz/handler/handler.go`

### Example Annotations

```go
// GetItems godoc
// @Summary Get all items
// @Description Retrieve a list of all listing items
// @Tags items
// @Produce json
// @Success 200 {object} httpHelper.Response{data=[]model.Item} "List of items"
// @Failure 500 {object} httpHelper.Response "Internal server error"
// @Router /items [get]
func (h *Handler) GetItems(w http.ResponseWriter, r *http.Request) {
    // ...
}
```

## Documentation Resources

- [Swaggo Documentation](https://github.com/swaggo/swag)
- [OpenAPI Specification](https://swagger.io/specification/)
- [Swagger UI](https://swagger.io/tools/swagger-ui/)
