// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package proxy

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/discovery/internal/thirdparty/module"
	"golang.org/x/net/context/ctxhttp"
)

// A Client is used by the fetch service to communicate with a module
// proxy. It handles all methods defined by go help goproxy.
type Client struct {
	// URL of the module proxy web server
	url string

	// client used for HTTP requests. It is mutable for testing purposes.
	httpClient *http.Client
}

// A VersionInfo contains metadata about a given version of a module.
type VersionInfo struct {
	Version string
	Time    time.Time
}

// cleanURL trims the rawurl of trailing slashes.
func cleanURL(rawurl string) string {
	return strings.TrimRight(rawurl, "/")
}

// New constructs a *Client using the provided rawurl, which is expected to
// be an absolute URI that can be directly passed to http.Get.
func New(rawurl string) (*Client, error) {
	url, err := url.Parse(rawurl)
	if err != nil {
		return nil, fmt.Errorf("url.Parse(%q): %v", rawurl, err)
	}
	if url.Scheme != "https" {
		return nil, fmt.Errorf("scheme must be https (got %s)", url.Scheme)
	}
	return &Client{url: cleanURL(rawurl), httpClient: http.DefaultClient}, nil
}

// infoURL constructs a url for a request to
// $GOPROXY/<module>/@v/list.
func (c *Client) infoURL(path, version string) (string, error) {
	encodedPath, encodedVersion, err := encodeModulePathAndVersion(path, version)
	if err != nil {
		return "", fmt.Errorf("encodeModulePathAndVersion(%q, %q): %v", path, version, err)
	}
	return fmt.Sprintf("%s/%s/@v/%s.info", c.url, encodedPath, encodedVersion), nil
}

// GetInfo makes a request to $GOPROXY/<module>/@v/<version>.info and
// transforms that data into a *VersionInfo.
func (c *Client) GetInfo(ctx context.Context, path, version string) (*VersionInfo, error) {
	u, err := c.infoURL(path, version)
	if err != nil {
		return nil, fmt.Errorf("infoURL(%q, %q): %v", path, version, err)
	}

	r, err := ctxhttp.Get(ctx, c.httpClient, u)
	if err != nil {
		return nil, fmt.Errorf("ctxhttp.Get(ctx, nil, %q): %v", u, err)
	}
	defer r.Body.Close()

	if r.StatusCode < 200 || r.StatusCode >= 300 {
		return nil, fmt.Errorf("ctxhttp.Get(ctx, %q) returned response: %d (%q)", u, r.StatusCode, r.Status)
	}

	var v VersionInfo
	if err = json.NewDecoder(r.Body).Decode(&v); err != nil {
		return nil, err
	}
	return &v, nil
}

// zipURL constructs a url for a request to $GOPROXY/<module>/@v/<version>.zip.
func (c *Client) zipURL(path, version string) (string, error) {
	encodedPath, encodedVersion, err := encodeModulePathAndVersion(path, version)
	if err != nil {
		return "", fmt.Errorf("encodeModulePathAndVersion(%q, %q): %v", path, version, err)
	}
	return fmt.Sprintf("%s/%s/@v/%s.zip", c.url, encodedPath, encodedVersion), nil
}

// GetZip makes a request to $GOPROXY/<module>/@v/<version>.zip and transforms
// that data into a *zip.Reader.
func (c *Client) GetZip(ctx context.Context, path, version string) (*zip.Reader, error) {
	u, err := c.zipURL(path, version)
	if err != nil {
		return nil, fmt.Errorf("zipURL(%q, %q): %v", path, version, err)
	}

	r, err := ctxhttp.Get(ctx, c.httpClient, u)
	if err != nil {
		return nil, fmt.Errorf("ctxhttp.Get(ctx, nil, %q): %v", u, err)
	}
	defer r.Body.Close()

	if r.StatusCode < 200 || r.StatusCode >= 300 {
		return nil, fmt.Errorf("get(ctx, %q) returned response: %d (%q)", u, r.StatusCode, r.Status)
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("get(ctx, %q): %v", u, err)
	}

	zipReader, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return nil, fmt.Errorf("get(ctx, %q): %v", u, err)
	}
	return zipReader, nil
}

func encodeModulePathAndVersion(path, version string) (string, string, error) {
	encodedPath, err := module.EncodePath(path)
	if err != nil {
		return "", "", fmt.Errorf("module.EncodePath(%q): %v", path, err)
	}
	encodedVersion, err := module.EncodeVersion(version)
	if err != nil {
		return "", "", fmt.Errorf("module.EncodeVersion(%q): %v", version, err)
	}
	return encodedPath, encodedVersion, nil
}
