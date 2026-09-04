package util

import (
	"regexp"
	"sort"
	"strings"
)

type Route struct {
	Pattern     string
	Element     string
	Title       string
	Description string
	Index       bool
}

type routeSegment struct {
	param bool
	value string
}

type compiledRoute struct {
	Route
	segments []routeSegment
}

type siteModel struct {
	routes    []compiledRoute
	templates map[string]string
	elements  map[string]string
}

var (
	templateOpenPattern = regexp.MustCompile(`(?is)<template\b([^>]*)>`)
	attrPattern         = regexp.MustCompile(`(?i)([a-z0-9:-]+)\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s>]+))`)
)

func parseTemplateRoutes(elementName string, fileContents []byte) []Route {
	open := templateOpenPattern.FindSubmatch(fileContents)
	if open == nil {
		return nil
	}
	attrs := parseAttrs(string(open[1]))
	id := attrs["id"]
	if id == "" {
		id = elementName
	}
	if id != elementName {
		return nil
	}
	raw := strings.TrimSpace(attrs["data-route"])
	if extra := strings.TrimSpace(attrs["data-routes"]); extra != "" {
		if raw != "" {
			raw += " " + extra
		} else {
			raw = extra
		}
	}
	if raw == "" {
		return nil
	}
	index := strings.Contains(strings.ToLower(string(open[0])), "data-index") && attrs["data-index"] != "false"

	var routes []Route
	for _, pattern := range strings.FieldsFunc(raw, func(r rune) bool {
		return r == ' ' || r == ','
	}) {
		pattern = normalizeRoutePattern(pattern)
		if pattern == "" {
			continue
		}
		routes = append(routes, Route{
			Pattern:     pattern,
			Element:     elementName,
			Title:       attrs["data-title"],
			Description: attrs["data-description"],
			Index:       index,
		})
	}
	return routes
}

func parseAttrs(raw string) map[string]string {
	attrs := map[string]string{}
	for _, match := range attrPattern.FindAllStringSubmatch(raw, -1) {
		value := match[2]
		if value == "" {
			value = match[3]
		}
		if value == "" {
			value = match[4]
		}
		attrs[strings.ToLower(match[1])] = value
	}
	return attrs
}

func normalizeRoutePattern(pattern string) string {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return ""
	}
	if pattern == "/" {
		return "/"
	}
	pattern = "/" + strings.Trim(pattern, "/")
	return pattern
}

func compileRoute(route Route) compiledRoute {
	pattern := normalizeRoutePattern(route.Pattern)
	var segments []routeSegment
	if pattern != "/" {
		for _, part := range strings.Split(strings.TrimPrefix(pattern, "/"), "/") {
			if strings.HasPrefix(part, ":") {
				segments = append(segments, routeSegment{param: true, value: strings.TrimPrefix(part, ":")})
				continue
			}
			segments = append(segments, routeSegment{value: part})
		}
	}
	route.Pattern = pattern
	return compiledRoute{Route: route, segments: segments}
}

func (site *siteModel) match(path string) (Route, map[string]string, bool) {
	path = normalizeRoutePattern(path)
	var parts []string
	if path != "/" {
		parts = strings.Split(strings.TrimPrefix(path, "/"), "/")
	}
	for _, route := range site.routes {
		if len(route.segments) != len(parts) {
			continue
		}
		params := map[string]string{}
		ok := true
		for i, segment := range route.segments {
			if segment.param {
				params[segment.value] = parts[i]
				continue
			}
			if segment.value != parts[i] {
				ok = false
				break
			}
		}
		if ok {
			return route.Route, params, true
		}
	}
	return Route{}, nil, false
}

func (site *siteModel) staticIndexPaths() []string {
	seen := map[string]struct{}{}
	var paths []string
	for _, route := range site.routes {
		if route.Index == false {
			continue
		}
		if strings.Contains(route.Pattern, ":") {
			continue
		}
		if _, ok := seen[route.Pattern]; ok {
			continue
		}
		seen[route.Pattern] = struct{}{}
		paths = append(paths, route.Pattern)
	}
	sort.Strings(paths)
	return paths
}
