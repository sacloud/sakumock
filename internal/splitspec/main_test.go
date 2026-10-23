package main

import (
	"strings"
	"testing"
)

const shared = `
openapi: 3.0.3
info:
  title: A/B API
  version: 1.0.0
security:
  - basicAuth: []
paths:
  /b/things:
    get:
      responses:
        "200":
          content:
            application/json:
              schema: {$ref: "#/components/schemas/Thing"}
  /a/items:
    get:
      parameters:
        - $ref: "#/components/parameters/Page"
      responses:
        "200":
          description: "<ok> & 日本語"
          content:
            application/json:
              schema: {$ref: "#/components/schemas/ItemList"}
components:
  schemas:
    Thing: {type: object}
    ItemList:
      type: array
      items: {$ref: "#/components/schemas/Item"}
    Item:
      type: object
      properties:
        created: {type: string, example: 2024-01-02}
        size: {type: integer, maximum: 10}
        on: {type: boolean, default: false}
  parameters:
    Page: {name: page, in: query, schema: {type: integer}}
  responses:
    Unused: {description: unused}
  securitySchemes:
    basicAuth: {type: http, scheme: basic}
`

const wantA = `{
  "openapi": "3.0.3",
  "info": {
    "title": "A/B API",
    "version": "1.0.0"
  },
  "security": [
    {
      "basicAuth": []
    }
  ],
  "paths": {
    "/a/items": {
      "get": {
        "parameters": [
          {
            "$ref": "#/components/parameters/Page"
          }
        ],
        "responses": {
          "200": {
            "description": "<ok> & 日本語",
            "content": {
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/ItemList"
                }
              }
            }
          }
        }
      }
    }
  },
  "components": {
    "schemas": {
      "ItemList": {
        "type": "array",
        "items": {
          "$ref": "#/components/schemas/Item"
        }
      },
      "Item": {
        "type": "object",
        "properties": {
          "created": {
            "type": "string",
            "example": "2024-01-02"
          },
          "size": {
            "type": "integer",
            "maximum": 10
          },
          "on": {
            "type": "boolean",
            "default": false
          }
        }
      }
    },
    "parameters": {
      "Page": {
        "name": "page",
        "in": "query",
        "schema": {
          "type": "integer"
        }
      }
    },
    "securitySchemes": {
      "basicAuth": {
        "type": "http",
        "scheme": "basic"
      }
    }
  }
}
`

func TestSplit(t *testing.T) {
	got, err := split([]byte(shared), []string{"/a/"})
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != wantA {
		t.Errorf("split mismatch:\n%s", got)
	}
}

func TestSplitMultiplePrefixes(t *testing.T) {
	got, err := split([]byte(shared), []string{"/a/", "/b/"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"/a/items"`, `"/b/things"`, `"Thing"`, `"Item"`} {
		if !strings.Contains(string(got), want) {
			t.Errorf("output lacks %s", want)
		}
	}
	if strings.Contains(string(got), `"Unused"`) {
		t.Error("unreferenced response kept")
	}
}

func TestSplitErrors(t *testing.T) {
	for name, tc := range map[string]struct {
		spec   string
		prefix string
		want   string
	}{
		"no match":   {shared, "/c/", "no paths match"},
		"unresolved": {strings.Replace(shared, "Thing: {type: object}", "", 1), "/b/", "unresolved $ref #/components/schemas/Thing"},
		"no paths":   {"openapi: 3.0.3\n", "/a/", "no paths"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := split([]byte(tc.spec), []string{tc.prefix})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v, want %q", err, tc.want)
			}
		})
	}
}
