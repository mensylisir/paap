package handler

import (
	"strings"
	"unicode"
)

const nativeComponentConfigTemplateSyntax = "普通模式使用 __TEMPLATE__KEY__显示名__ 标记原生配置中的变量；支持 DEFAULT、IF、FOR。高级模式可导入 PAAP JSON / schema。"

type nativeComponentConfigTemplateOptions struct {
	Framework string
	FileName  string
}

type nativeParsedComponentConfigTemplate struct {
	fields        []map[string]interface{}
	env           []map[string]interface{}
	configMaps    []map[string]interface{}
	secrets       []map[string]interface{}
	files         []map[string]interface{}
	nativeConfigs []map[string]interface{}
}

type nativeBlockFrame struct {
	kind string
	key  string
}

func parseNativeComponentConfigTemplate(source string, options nativeComponentConfigTemplateOptions) nativeParsedComponentConfigTemplate {
	framework := strings.ToLower(strings.TrimSpace(options.Framework))
	fileName := strings.TrimSpace(options.FileName)
	if fileName == "" {
		fileName = defaultNativeTemplateFileName(framework)
	}

	fields := make([]map[string]interface{}, 0)
	fieldSeen := map[string]bool{}
	listIndexes := map[string]int{}
	stack := make([]nativeBlockFrame, 0)
	var output strings.Builder

	for cursor := 0; cursor < len(source); {
		idx := strings.Index(source[cursor:], "__TEMPLATE__")
		if idx < 0 {
			output.WriteString(source[cursor:])
			break
		}
		idx += cursor
		output.WriteString(source[cursor:idx])

		token, next, ok := readNativeTemplateToken(source, idx)
		if !ok {
			output.WriteString(source[idx : idx+len("__TEMPLATE__")])
			cursor = idx + len("__TEMPLATE__")
			continue
		}
		cursor = next

		switch token.kind {
		case "FOR":
			stack = append(stack, nativeBlockFrame{kind: "FOR", key: token.key})
			if _, exists := listIndexes[token.key]; !exists {
				listIndexes[token.key] = len(fields)
				fields = append(fields, map[string]interface{}{
					"key":        token.key,
					"label":      firstNonEmpty(token.label, labelFromNativeTemplateKey(token.key)),
					"type":       "list",
					"itemFields": []map[string]interface{}{},
				})
			}
			output.WriteString("[[paap:for " + token.key + "]]")
		case "IF":
			stack = append(stack, nativeBlockFrame{kind: "IF", key: token.key})
			addNativeTemplateField(&fields, fieldSeen, inferNativeTemplateField(token.key, token.label, token.defaultValue, false))
			output.WriteString("[[paap:if " + token.key + "]]")
		case "END":
			key := token.key
			if len(stack) > 0 {
				key = firstNonEmpty(key, stack[len(stack)-1].key)
				stack = stack[:len(stack)-1]
			}
			output.WriteString("[[paap:end " + key + "]]")
		case "VALUE":
			if strings.HasPrefix(token.key, "ITEM_") {
				listKey := currentNativeTemplateListKey(stack)
				itemKey := strings.TrimPrefix(token.key, "ITEM_")
				idx, exists := listIndexes[listKey]
				if !exists {
					idx = len(fields)
					listIndexes[listKey] = idx
					fields = append(fields, map[string]interface{}{
						"key":        listKey,
						"label":      labelFromNativeTemplateKey(listKey),
						"type":       "list",
						"itemFields": []map[string]interface{}{},
					})
				}
				itemFields, _ := fields[idx]["itemFields"].([]map[string]interface{})
				if !nativeTemplateFieldExists(itemFields, itemKey) {
					itemFields = append(itemFields, inferNativeTemplateField(itemKey, token.label, token.defaultValue, true))
					fields[idx]["itemFields"] = itemFields
				}
				output.WriteString("[[paap:item." + itemKey + nativeTemplateDefaultOption(token.defaultValue) + "]]")
				continue
			}
			addNativeTemplateField(&fields, fieldSeen, inferNativeTemplateField(token.key, token.label, token.defaultValue, false))
			output.WriteString("[[paap:" + token.key + nativeTemplateDefaultOption(token.defaultValue) + "]]")
		}
	}

	file := map[string]interface{}{
		"name":                 fileName,
		"configMapName":        "{{configMapName}}",
		"key":                  fileName,
		"recommendedMountPath": defaultNativeTemplateMountPath(framework, fileName),
		"readOnly":             true,
	}
	return nativeParsedComponentConfigTemplate{
		fields: fields,
		env:    defaultNativeTemplateEnv(framework),
		configMaps: []map[string]interface{}{
			{"name": "{{configMapName}}", "data": map[string]string{fileName: output.String()}},
		},
		secrets:       []map[string]interface{}{},
		files:         []map[string]interface{}{file},
		nativeConfigs: []map[string]interface{}{{"name": fileName, "content": source}},
	}
}

type nativeTemplateToken struct {
	kind         string
	key          string
	label        string
	defaultValue string
}

func readNativeTemplateToken(source string, start int) (nativeTemplateToken, int, bool) {
	const prefix = "__TEMPLATE__"
	body := source[start+len(prefix):]
	for _, kind := range []string{"FOR", "IF", "END"} {
		kindPrefix := kind + "__"
		if strings.HasPrefix(body, kindPrefix) {
			key, pos, ok := readNativeTemplatePart(body, len(kindPrefix))
			if !ok {
				return nativeTemplateToken{}, start, false
			}
			label := ""
			if kind != "END" {
				var labelOK bool
				label, pos, labelOK = readNativeTemplatePart(body, pos)
				if !labelOK {
					return nativeTemplateToken{}, start, false
				}
			}
			return nativeTemplateToken{kind: kind, key: key, label: label}, start + len(prefix) + pos, true
		}
	}

	key, pos, ok := readNativeTemplatePart(body, 0)
	if !ok {
		return nativeTemplateToken{}, start, false
	}
	label, pos, ok := readNativeTemplatePart(body, pos)
	if !ok {
		return nativeTemplateToken{}, start, false
	}
	defaultValue := ""
	if strings.HasPrefix(body[pos:], "DEFAULT__") {
		var defaultOK bool
		defaultValue, pos, defaultOK = readNativeTemplatePart(body, pos+len("DEFAULT__"))
		if !defaultOK {
			return nativeTemplateToken{}, start, false
		}
	}
	return nativeTemplateToken{kind: "VALUE", key: key, label: label, defaultValue: defaultValue}, start + len(prefix) + pos, true
}

func readNativeTemplatePart(source string, start int) (string, int, bool) {
	end := strings.Index(source[start:], "__")
	if end < 0 {
		return "", start, false
	}
	end += start
	return strings.TrimSpace(source[start:end]), end + len("__"), true
}

func addNativeTemplateField(fields *[]map[string]interface{}, seen map[string]bool, field map[string]interface{}) {
	key, _ := field["key"].(string)
	if key == "" || seen[key] {
		return
	}
	seen[key] = true
	*fields = append(*fields, field)
}

func inferNativeTemplateField(key, label, defaultValue string, item bool) map[string]interface{} {
	upper := strings.ToUpper(strings.TrimSpace(key))
	field := map[string]interface{}{
		"key":   strings.TrimSpace(key),
		"label": firstNonEmpty(strings.TrimSpace(label), labelFromNativeTemplateKey(key)),
		"type":  inferNativeTemplateFieldType(upper),
	}
	if defaultValue != "" {
		field["default"] = defaultValue
	}
	if strings.Contains(upper, "PASSWORD") || strings.Contains(upper, "SECRET") || strings.Contains(upper, "TOKEN") || strings.Contains(upper, "PRIVATE_KEY") || strings.Contains(upper, "ACCESS_KEY") {
		field["type"] = "password"
		field["output"] = "secret"
		field["sensitive"] = true
	}
	if target := inferNativeTemplateServiceTarget(upper); target != "" {
		field["type"] = "serviceRef"
		field["target"] = target
		if format := inferNativeTemplateServiceFormat(upper); format != "" {
			field["format"] = format
		}
	}
	return field
}

func inferNativeTemplateFieldType(key string) string {
	if strings.HasSuffix(key, "PORT") || strings.Contains(key, "_PORT_") {
		return "number"
	}
	if strings.HasSuffix(key, "ENABLED") || strings.HasSuffix(key, "ENABLE") || strings.HasPrefix(key, "ENABLE_") || strings.HasPrefix(key, "USE_") {
		return "boolean"
	}
	return "text"
}

func inferNativeTemplateServiceTarget(key string) string {
	if strings.HasSuffix(key, "USER") || strings.HasSuffix(key, "USERNAME") || strings.HasSuffix(key, "PASSWORD") || strings.HasSuffix(key, "SECRET") || strings.HasSuffix(key, "TOKEN") || strings.HasSuffix(key, "KEY") {
		return ""
	}
	if strings.Contains(key, "PROXY_PASS") {
		return "backend"
	}
	if strings.Contains(key, "BACKEND") && nativeTemplateHasAddressToken(key) {
		return "backend"
	}
	if nativeTemplateHasAny(key, "JDBC", "DATABASE", "DATASOURCE", "POSTGRES", "MYSQL") && nativeTemplateHasAddressToken(key) {
		return "postgresql|mysql"
	}
	if strings.Contains(key, "REDIS") && nativeTemplateHasAddressToken(key) {
		return "redis"
	}
	if nativeTemplateHasAny(key, "RABBIT", "MQ") && nativeTemplateHasAddressToken(key) {
		return "rabbitmq"
	}
	if strings.Contains(key, "KAFKA") && nativeTemplateHasAddressToken(key) {
		return "kafka"
	}
	if nativeTemplateHasAny(key, "MINIO", "S3") && nativeTemplateHasAddressToken(key) {
		return "minio"
	}
	return ""
}

func inferNativeTemplateServiceFormat(key string) string {
	if strings.Contains(key, "JDBC") {
		return "jdbcUrl"
	}
	if strings.HasSuffix(key, "HOST") || strings.HasSuffix(key, "HOSTNAME") {
		return "host"
	}
	if strings.HasSuffix(key, "PORT") {
		return "port"
	}
	if strings.HasSuffix(key, "ADDR") || strings.HasSuffix(key, "ADDRESS") || strings.HasSuffix(key, "BOOTSTRAP") || strings.HasSuffix(key, "BOOTSTRAP_SERVERS") {
		return "addr"
	}
	if strings.HasSuffix(key, "URL") || strings.HasSuffix(key, "URI") || strings.HasSuffix(key, "DSN") {
		return "url"
	}
	return ""
}

func nativeTemplateHasAddressToken(key string) bool {
	return nativeTemplateHasAny(key, "URL", "URI", "HOST", "ADDR", "ADDRESS", "JDBC", "ENDPOINT", "BOOTSTRAP")
}

func nativeTemplateHasAny(value string, tokens ...string) bool {
	for _, token := range tokens {
		if strings.Contains(value, token) {
			return true
		}
	}
	return false
}

func nativeTemplateDefaultOption(value string) string {
	if value == "" {
		return ""
	}
	return " default=" + value
}

func currentNativeTemplateListKey(stack []nativeBlockFrame) string {
	for i := len(stack) - 1; i >= 0; i-- {
		if stack[i].kind == "FOR" && stack[i].key != "" {
			return stack[i].key
		}
	}
	return "ITEMS"
}

func nativeTemplateFieldExists(fields []map[string]interface{}, key string) bool {
	for _, field := range fields {
		if field["key"] == key {
			return true
		}
	}
	return false
}

func labelFromNativeTemplateKey(key string) string {
	parts := strings.FieldsFunc(strings.ToLower(key), func(r rune) bool {
		return r == '_' || r == '-' || r == '.'
	})
	for i, part := range parts {
		if part == "" {
			continue
		}
		runes := []rune(part)
		runes[0] = unicode.ToUpper(runes[0])
		parts[i] = string(runes)
	}
	return strings.Join(parts, " ")
}

func defaultNativeTemplateFileName(framework string) string {
	switch framework {
	case "nginx":
		return "default.conf"
	case "spring", "springboot":
		return "application-paap.yml"
	case "node", "go", "python":
		return ".env"
	default:
		return "application.conf"
	}
}

func defaultNativeTemplateMountPath(framework, fileName string) string {
	switch framework {
	case "nginx":
		return "/etc/nginx/conf.d/default.conf"
	case "spring", "springboot":
		return "/etc/paap/" + fileName
	default:
		return "/etc/paap/" + fileName
	}
}

func defaultNativeTemplateEnv(framework string) []map[string]interface{} {
	if framework == "spring" || framework == "springboot" {
		return []map[string]interface{}{{"name": "SPRING_CONFIG_ADDITIONAL_LOCATION", "source": "value", "value": "file:/etc/paap/"}}
	}
	return []map[string]interface{}{}
}
