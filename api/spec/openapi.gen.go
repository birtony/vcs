// Package spec provides primitives to interact with the openapi HTTP API.
//
// Code generated by github.com/deepmap/oapi-codegen version v1.11.0 DO NOT EDIT.
package spec

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"net/url"
	"path"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	externalRef0 "github.com/trustbloc/vcs/pkg/restapi/v1/common"
)

// Base64 encoded, gzipped, json marshaled Swagger object
var swaggerSpec = []string{

	"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",
}

// GetSwagger returns the content of the embedded swagger specification file
// or error if failed to decode
func decodeSpec() ([]byte, error) {
	zipped, err := base64.StdEncoding.DecodeString(strings.Join(swaggerSpec, ""))
	if err != nil {
		return nil, fmt.Errorf("error base64 decoding spec: %s", err)
	}
	zr, err := gzip.NewReader(bytes.NewReader(zipped))
	if err != nil {
		return nil, fmt.Errorf("error decompressing spec: %s", err)
	}
	var buf bytes.Buffer
	_, err = buf.ReadFrom(zr)
	if err != nil {
		return nil, fmt.Errorf("error decompressing spec: %s", err)
	}

	return buf.Bytes(), nil
}

var rawSpec = decodeSpecCached()

// a naive cached of a decoded swagger spec
func decodeSpecCached() func() ([]byte, error) {
	data, err := decodeSpec()
	return func() ([]byte, error) {
		return data, err
	}
}

// Constructs a synthetic filesystem for resolving external references when loading openapi specifications.
func PathToRawSpec(pathToFile string) map[string]func() ([]byte, error) {
	var res = make(map[string]func() ([]byte, error))
	if len(pathToFile) > 0 {
		res[pathToFile] = rawSpec
	}

	pathPrefix := path.Dir(pathToFile)

	for rawPath, rawFunc := range externalRef0.PathToRawSpec(path.Join(pathPrefix, "./common.yaml")) {
		if _, ok := res[rawPath]; ok {
			// it is not possible to compare functions in golang, so always overwrite the old value
		}
		res[rawPath] = rawFunc
	}
	return res
}

// GetSwagger returns the Swagger specification corresponding to the generated code
// in this file. The external references of Swagger specification are resolved.
// The logic of resolving external references is tightly connected to "import-mapping" feature.
// Externally referenced files must be embedded in the corresponding golang packages.
// Urls can be supported but this task was out of the scope.
func GetSwagger() (swagger *openapi3.T, err error) {
	var resolvePath = PathToRawSpec("")

	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true
	loader.ReadFromURIFunc = func(loader *openapi3.Loader, url *url.URL) ([]byte, error) {
		var pathToFile = url.String()
		pathToFile = path.Clean(pathToFile)
		getSpec, ok := resolvePath[pathToFile]
		if !ok {
			err1 := fmt.Errorf("path not found: %s", pathToFile)
			return nil, err1
		}
		return getSpec()
	}
	var specData []byte
	specData, err = rawSpec()
	if err != nil {
		return
	}
	swagger, err = loader.LoadFromData(specData)
	if err != nil {
		return
	}
	return
}
