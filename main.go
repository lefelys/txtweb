package main

import (
	"context"
	"errors"
	"log"
	"mime"
	"net"
	"net/http"
	"strings"
	"time"
)

type txtResolver interface {
	LookupTXT(ctx context.Context, host string) ([]string, error)
}

type serveFunc func(server *http.Server) error

const (
	txtwebRecord              = "_txtweb"
	txtwebConfigRecord        = "_txtweb_cfg"
	defaultPlainContentType   = "text/plain; charset=UTF-8"
	defaultWrappedContentType = "text/html; charset=UTF-8"
	defaultHTMLPadding        = "24px"
	poweredByHeaderName       = "X-Powered-By"
	poweredByHeaderValue      = "txtweb; served from DNS TXT record, see https://txtweb.lefelys.com"
)

const indexHeader = `

█████████████████████████████████████████
█─▄─▄─█▄─▀─▄█─▄─▄─█▄─█▀▀▀█─▄█▄─▄▄─█▄─▄─▀█
███─████▀─▀████─████─█─█─█─███─▄█▀██─▄─▀█
▀▀▄▄▄▀▀▄▄█▄▄▀▀▄▄▄▀▀▀▄▄▄▀▄▄▄▀▀▄▄▄▄▄▀▄▄▄▄▀▀

txtweb — serve a website from a DNS TXT record.

Info: https://txtweb.lefelys.com
`

func resolveTXTRecord(ctx context.Context, resolver txtResolver, name string) ([]string, error) {
	records, err := resolver.LookupTXT(ctx, name)
	if err != nil {
		var dnsErr *net.DNSError
		if errors.As(err, &dnsErr) && dnsErr.IsNotFound {
			return nil, nil
		}
		return nil, err
	}

	if len(records) == 0 {
		return nil, nil
	}

	txtRecords := make([]string, 0, len(records))
	for _, record := range records {
		trimmed := strings.TrimSpace(record)
		if trimmed != "" {
			txtRecords = append(txtRecords, trimmed)
		}
	}

	if len(txtRecords) == 0 {
		return nil, nil
	}

	return txtRecords, nil
}

func lookupFirstTXTRecord(ctx context.Context, resolver txtResolver, record, hostname string) (string, error) {
	records, err := resolveTXTRecord(ctx, resolver, record+"."+hostname)
	if err != nil || len(records) == 0 {
		return "", err
	}
	return strings.TrimSpace(records[0]), nil
}

func parseTXTWebConfig(cfg string) map[string]string {
	result := map[string]string{}
	trimmed := strings.TrimSpace(cfg)
	if trimmed == "" {
		return result
	}

	_, params, err := mime.ParseMediaType("text/plain; " + trimmed)
	if err != nil {
		return result
	}

	for key, value := range params {
		normalizedKey := strings.ToLower(strings.TrimSpace(key))
		normalizedValue := strings.TrimSpace(value)
		if normalizedKey != "" && normalizedValue != "" {
			result[normalizedKey] = normalizedValue
		}
	}

	return result
}

func wrapHTML(content, align, maxWidth, bgColor, fgColor string) string {
	bodyStyles := []string{"margin:0", "min-height:100vh", "padding:" + defaultHTMLPadding, "box-sizing:border-box"}
	contentStyles := []string{"white-space:pre-wrap", "text-align:left"}

	switch align {
	case "top-right":
		bodyStyles = append(bodyStyles, "display:flex", "align-items:flex-start", "justify-content:flex-end")
	case "bottom-left":
		bodyStyles = append(bodyStyles, "display:flex", "align-items:flex-end", "justify-content:flex-start")
	case "bottom-right":
		bodyStyles = append(bodyStyles, "display:flex", "align-items:flex-end", "justify-content:flex-end")
	case "center":
		bodyStyles = append(bodyStyles, "display:flex", "align-items:center", "justify-content:center")
	default:
		bodyStyles = append(bodyStyles, "display:flex", "align-items:flex-start", "justify-content:flex-start")
	}

	if bgColor != "" {
		bodyStyles = append(bodyStyles, "background:"+bgColor)
	}
	if fgColor != "" {
		bodyStyles = append(bodyStyles, "color:"+fgColor)
	}
	if maxWidth != "" {
		contentStyles = append(contentStyles, "max-width:"+maxWidth, "width:100%")
	}

	safeHeaderComment := strings.ReplaceAll(indexHeader, "--", "—")
	return "<!--\n" + safeHeaderComment + "\n-->\n<!doctype html><html><head><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width, initial-scale=1\"></head><body style=\"" +
		strings.Join(bodyStyles, ";") +
		"\"><div style=\"" +
		strings.Join(contentStyles, ";") +
		"\">" +
		content +
		"</div></body></html>"
}

func extractHostname(host string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return ""
	}

	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		return strings.Trim(parsedHost, "[]")
	}

	// Likely IPv6 without a port.
	if strings.Count(host, ":") > 1 {
		return strings.Trim(host, "[]")
	}

	return host
}

func newHandler(resolver txtResolver) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(poweredByHeaderName, poweredByHeaderValue)

		hostHeader := r.Host
		if hostHeader == "" {
			hostHeader = r.URL.Host
		}

		hostname := extractHostname(hostHeader)
		if hostname == "" {
			http.Error(w, "Missing Host header", http.StatusBadRequest)
			return
		}

		txtRecords, err := resolveTXTRecord(r.Context(), resolver, txtwebRecord+"."+hostname)
		if err != nil {
			http.Error(w, "DNS lookup failed", http.StatusInternalServerError)
			return
		}

		if len(txtRecords) == 0 {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(indexHeader))
			return
		}

		cfgRecord, err := lookupFirstTXTRecord(r.Context(), resolver, txtwebConfigRecord, hostname)
		if err != nil {
			http.Error(w, "DNS lookup failed", http.StatusInternalServerError)
			return
		}

		cfg := parseTXTWebConfig(cfgRecord)
		contentType := cfg["content-type"]
		wrapValue := cfg["html-wrap"]
		alignValue := cfg["html-align"]
		maxWidthValue := cfg["html-max-width"]
		bgColorValue := cfg["html-bg"]
		fgColorValue := cfg["html-fg"]

		responseBody := strings.Join(txtRecords, "\n")
		if strings.EqualFold(strings.TrimSpace(wrapValue), "true") {
			responseBody = wrapHTML(
				responseBody,
				alignValue,
				maxWidthValue,
				bgColorValue,
				fgColorValue,
			)
			if contentType == "" {
				contentType = defaultWrappedContentType
			}
		}

		if contentType == "" {
			contentType = defaultPlainContentType
		}

		w.Header().Set("content-type", contentType)
		_, _ = w.Write([]byte(responseBody))
	})
}

func runWith(resolver txtResolver, serve serveFunc) error {
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	if serve == nil {
		serve = func(server *http.Server) error {
			return server.ListenAndServe()
		}
	}

	server := &http.Server{
		Addr:              ":80",
		Handler:           newHandler(resolver),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("listening on %s", server.Addr)
	if err := serve(server); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}

func main() {
	if err := runWith(net.DefaultResolver, nil); err != nil {
		log.Fatal(err)
	}
}
