package openapi

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	v2high "github.com/pb33f/libopenapi/datamodel/high/v2"
	v3high "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/shayan-shojaei/radar/pkg/models"
	"go.yaml.in/yaml/v4"
)

// fetchSpec retrieves raw spec bytes from a URL or local file path.
func fetchSpec(source string) ([]byte, error) {
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Get(source)
		if err != nil {
			return nil, fmt.Errorf("openapi: fetch spec: %w", err)
		}
		defer resp.Body.Close()
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("openapi: read response body: %w", err)
		}
		return data, nil
	}
	data, err := os.ReadFile(source)
	if err != nil {
		return nil, fmt.Errorf("openapi: read file: %w", err)
	}
	return data, nil
}

// Parse fetches and parses an OpenAPI/Swagger spec, returning normalized endpoints and the base URL.
func Parse(source string) ([]models.ParsedEndpoint, string, error) {
	data, err := fetchSpec(source)
	if err != nil {
		return nil, "", err
	}

	doc, err := libopenapi.NewDocument(data)
	if err != nil {
		return nil, "", fmt.Errorf("openapi: parse document: %w", err)
	}

	switch doc.GetSpecInfo().SpecType {
	case "swagger":
		return parseSwagger(doc)
	default:
		return parseOpenAPI3(doc)
	}
}

func parseSwagger(doc libopenapi.Document) ([]models.ParsedEndpoint, string, error) {
	model, err := doc.BuildV2Model()
	if err != nil {
		return nil, "", fmt.Errorf("openapi: build swagger model: %w", err)
	}

	sw := &model.Model
	baseURL := swaggerBaseURL(sw)

	var endpoints []models.ParsedEndpoint

	if sw.Paths == nil || sw.Paths.PathItems == nil {
		return endpoints, baseURL, nil
	}

	for path, item := range sw.Paths.PathItems.FromOldest() {
		type opEntry struct {
			method string
			op     *v2high.Operation
		}
		ops := []opEntry{
			{"GET", item.Get},
			{"POST", item.Post},
			{"PUT", item.Put},
			{"DELETE", item.Delete},
			{"PATCH", item.Patch},
			{"OPTIONS", item.Options},
			{"HEAD", item.Head},
		}

		for _, o := range ops {
			if o.op == nil {
				continue
			}

			ep := models.ParsedEndpoint{
				Method:      o.method,
				Path:        path,
				Summary:     o.op.Summary,
				Description: o.op.Description,
				Tags:        o.op.Tags,
			}

			for _, p := range o.op.Parameters {
				param := models.Parameter{
					Name: p.Name,
					In:   p.In,
				}
				if p.Required != nil {
					param.Required = *p.Required
				}
				if p.Schema != nil {
					schema := p.Schema.Schema()
					if schema != nil && len(schema.Type) > 0 {
						param.Schema = schema.Type[0]
					}
				}
				ep.Parameters = append(ep.Parameters, param)

				if strings.EqualFold(p.In, "body") {
					ep.HasBody = true
					ep.BodyFields = extractBodyFields(p.Schema)
				}
			}

			endpoints = append(endpoints, ep)
		}
	}

	return endpoints, baseURL, nil
}

func parseOpenAPI3(doc libopenapi.Document) ([]models.ParsedEndpoint, string, error) {
	model, err := doc.BuildV3Model()
	if err != nil {
		return nil, "", fmt.Errorf("openapi: build openapi3 model: %w", err)
	}

	d := &model.Model
	baseURL := openAPI3BaseURL(d)

	var endpoints []models.ParsedEndpoint

	if d.Paths == nil || d.Paths.PathItems == nil {
		return endpoints, baseURL, nil
	}

	for path, item := range d.Paths.PathItems.FromOldest() {
		type opEntry struct {
			method string
			op     *v3high.Operation
		}
		ops := []opEntry{
			{"GET", item.Get},
			{"POST", item.Post},
			{"PUT", item.Put},
			{"DELETE", item.Delete},
			{"PATCH", item.Patch},
			{"OPTIONS", item.Options},
			{"HEAD", item.Head},
		}

		for _, o := range ops {
			if o.op == nil {
				continue
			}

			ep := models.ParsedEndpoint{
				Method:      o.method,
				Path:        path,
				Summary:     o.op.Summary,
				Description: o.op.Description,
				Tags:        o.op.Tags,
				HasBody:     o.op.RequestBody != nil,
			}

			for _, p := range o.op.Parameters {
				param := models.Parameter{
					Name: p.Name,
					In:   p.In,
				}
				if p.Required != nil {
					param.Required = *p.Required
				}
				if p.Schema != nil {
					schema := p.Schema.Schema()
					if schema != nil && len(schema.Type) > 0 {
						param.Schema = schema.Type[0]
					}
				}
				ep.Parameters = append(ep.Parameters, param)
			}

			if o.op.RequestBody != nil && o.op.RequestBody.Content != nil {
				for contentType, mt := range o.op.RequestBody.Content.FromOldest() {
					if contentType == "application/json" && mt != nil {
						ep.BodyFields = extractBodyFields(mt.Schema)
						break
					}
				}
			}

			endpoints = append(endpoints, ep)
		}
	}

	return endpoints, baseURL, nil
}

func swaggerBaseURL(sw *v2high.Swagger) string {
	if sw == nil {
		return ""
	}
	scheme := "https"
	if len(sw.Schemes) > 0 {
		scheme = sw.Schemes[0]
	}
	return fmt.Sprintf("%s://%s%s", scheme, sw.Host, sw.BasePath)
}

func openAPI3BaseURL(d *v3high.Document) string {
	if d == nil || len(d.Servers) == 0 {
		return ""
	}
	return d.Servers[0].URL
}

func extractBodyFields(sp *base.SchemaProxy) []models.BodyField {
	if sp == nil {
		return nil
	}
	schema := sp.Schema()
	if schema == nil || schema.Properties == nil {
		return nil
	}
	required := make(map[string]bool)
	for _, r := range schema.Required {
		required[r] = true
	}
	var fields []models.BodyField
	for name, prop := range schema.Properties.FromOldest() {
		bf := models.BodyField{Name: name, Required: required[name]}
		if ps := prop.Schema(); ps != nil {
			if len(ps.Type) > 0 {
				bf.Type = ps.Type[0]
			}
			if ps.Example != nil && ps.Example.Kind == yaml.ScalarNode {
				bf.Example = ps.Example.Value
			}
		}
		fields = append(fields, bf)
	}
	return fields
}
