package service

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"paap/internal/model"
)

const ComponentConfigTemplateSyntax = `Use [[paap:<field>]] placeholders in env values or config files. Common options: default=<value>, secret=true, output=configMap|secret. PAAP also supports {{componentName}}, {{configMapName}}, and {{secretName}} runtime tokens.`

const NativeComponentConfigTemplateSyntax = "普通模式上传用户自己的配置文件，并使用 __TEMPLATE__KEY__显示名__ 标记需要填写的字段；支持 DEFAULT、IF、FOR。template.json/schema.json 只在高级模板包模式解析。"

type ComponentConfigTemplateInput struct {
	Key            string
	Name           string
	Description    string
	Framework      string
	BindingMode    string
	ComponentTypes []string
	S3Bucket       string
	S3Key          string
	Syntax         string
	NativeConfigs  []map[string]interface{}
	Fields         []map[string]interface{}
	Env            []map[string]interface{}
	ConfigMaps     []map[string]interface{}
	Secrets        []map[string]interface{}
	Files          []map[string]interface{}
	Command        []string
	Args           []string
	Enabled        *bool
}

type ComponentConfigTemplateUploadOptions struct {
	Key            string
	Name           string
	Description    string
	Framework      string
	BindingMode    string
	Mode           string
	FileName       string
	ComponentTypes []string
}

type NativeComponentConfigTemplateFile struct {
	Name    string
	Content string
}

type NativeComponentConfigTemplateOptions struct {
	Framework string
	FileName  string
}

type NativeParsedComponentConfigTemplate struct {
	Fields        []map[string]interface{}
	Env           []map[string]interface{}
	ConfigMaps    []map[string]interface{}
	Secrets       []map[string]interface{}
	Files         []map[string]interface{}
	NativeConfigs []map[string]interface{}
}

type componentConfigTemplatePackage struct {
	Key            string                   `json:"key"`
	Name           string                   `json:"name"`
	Description    string                   `json:"description"`
	Framework      string                   `json:"framework"`
	BindingMode    string                   `json:"bindingMode"`
	ComponentTypes []string                 `json:"componentTypes"`
	SortOrder      int                      `json:"sortOrder"`
	Syntax         string                   `json:"syntax"`
	NativeConfigs  []map[string]interface{} `json:"nativeConfigs"`
	Fields         []map[string]interface{} `json:"fields"`
	Env            []map[string]interface{} `json:"env"`
	ConfigMaps     []map[string]interface{} `json:"configMaps"`
	Secrets        []map[string]interface{} `json:"secrets"`
	Files          []map[string]interface{} `json:"files"`
	Command        []string                 `json:"command"`
	Args           []string                 `json:"args"`
	Enabled        *bool                    `json:"enabled"`
}

func ComponentConfigTemplateFromInput(input ComponentConfigTemplateInput) (model.ComponentConfigTemplate, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return model.ComponentConfigTemplate{}, fmt.Errorf("template name is required")
	}
	key := strings.TrimSpace(input.Key)
	if key == "" {
		key = "custom-" + componentConfigTemplateKeySuffix(name)
	}
	if strings.Trim(key, "-") == "" {
		return model.ComponentConfigTemplate{}, fmt.Errorf("template key is required")
	}
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	syntax := strings.TrimSpace(input.Syntax)
	if syntax == "" {
		syntax = ComponentConfigTemplateSyntax
	}
	return model.ComponentConfigTemplate{
		Key:            key,
		Name:           name,
		Description:    strings.TrimSpace(input.Description),
		Framework:      firstNonEmpty(strings.TrimSpace(input.Framework), "auto"),
		BindingMode:    firstNonEmpty(strings.TrimSpace(input.BindingMode), "recommended"),
		ComponentTypes: MustJSON(input.ComponentTypes),
		S3Bucket:       strings.TrimSpace(input.S3Bucket),
		S3Key:          strings.TrimSpace(input.S3Key),
		Syntax:         syntax,
		NativeJSON:     MustJSON(input.NativeConfigs),
		FieldsJSON:     MustJSON(input.Fields),
		EnvJSON:        MustJSON(input.Env),
		ConfigJSON:     MustJSON(input.ConfigMaps),
		SecretJSON:     MustJSON(input.Secrets),
		FileJSON:       MustJSON(NormalizeComponentConfigTemplateFileHints(input.Files)),
		CommandJSON:    MustJSON(input.Command),
		ArgsJSON:       MustJSON(input.Args),
		IsBuiltin:      false,
		SortOrder:      1000,
		Enabled:        enabled,
	}, nil
}

func BuiltInComponentConfigTemplateArchivePaths() ([]string, error) {
	for _, dir := range []string{
		"/config-templates",
		filepath.Join("data", "config-templates"),
		filepath.Join("..", "..", "data", "config-templates"),
	} {
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			continue
		}
		paths, err := filepath.Glob(filepath.Join(dir, "*.tar.gz"))
		if err != nil {
			return nil, err
		}
		if len(paths) > 0 {
			return paths, nil
		}
	}
	return nil, fmt.Errorf("no built-in component config templates found")
}

func ParseComponentConfigTemplatePackageFile(filePath string) (model.ComponentConfigTemplate, error) {
	entries, err := readComponentConfigTemplatePackage(filePath)
	if err != nil {
		return model.ComponentConfigTemplate{}, err
	}
	templateRaw, ok := entries["template.json"]
	if !ok {
		return model.ComponentConfigTemplate{}, fmt.Errorf("template package must contain template.json")
	}
	var root map[string]interface{}
	if err := json.Unmarshal(templateRaw, &root); err != nil {
		return model.ComponentConfigTemplate{}, fmt.Errorf("parse template.json: %w", err)
	}
	if nested, ok := root["template"].(map[string]interface{}); ok {
		root = nested
	}
	var pkg componentConfigTemplatePackage
	encoded, _ := json.Marshal(root)
	if err := json.Unmarshal(encoded, &pkg); err != nil {
		return model.ComponentConfigTemplate{}, fmt.Errorf("decode template.json: %w", err)
	}

	if schemaRaw, ok := entries["schema.json"]; ok {
		var schema struct {
			Fields []map[string]interface{} `json:"fields"`
		}
		if err := json.Unmarshal(schemaRaw, &schema); err != nil {
			return model.ComponentConfigTemplate{}, fmt.Errorf("parse schema.json: %w", err)
		}
		if len(schema.Fields) > 0 {
			pkg.Fields = schema.Fields
		}
	}

	framework := firstNonEmpty(strings.TrimSpace(pkg.Framework), "auto")
	generatedConfigMaps := make([]map[string]interface{}, 0)
	generatedFiles := make([]map[string]interface{}, 0)
	nativeConfigs := make([]map[string]interface{}, 0, len(pkg.NativeConfigs))
	for _, item := range pkg.NativeConfigs {
		name := firstNonEmpty(templateMapString(item, "name"), path.Base(templateMapString(item, "path")))
		sourcePath := cleanTemplatePackagePath(templateMapString(item, "path"))
		content := templateMapString(item, "content")
		if sourcePath != "" {
			if data, ok := entries[sourcePath]; ok {
				content = string(data)
			} else {
				return model.ComponentConfigTemplate{}, fmt.Errorf("native config file %s not found", sourcePath)
			}
		}
		if strings.TrimSpace(name) == "" {
			name = defaultNativeTemplateFileName(framework)
		}
		if content == "" {
			continue
		}
		parsed := ParseNativeComponentConfigTemplate(content, NativeComponentConfigTemplateOptions{Framework: framework, FileName: name})
		if len(pkg.Fields) == 0 {
			pkg.Fields = parsed.Fields
		}
		generatedConfigMaps = append(generatedConfigMaps, parsed.ConfigMaps...)
		generatedFiles = append(generatedFiles, parsed.Files...)
		nativeConfigs = append(nativeConfigs, map[string]interface{}{"name": name, "content": content})
	}
	if len(nativeConfigs) > 0 {
		pkg.NativeConfigs = nativeConfigs
	}
	if len(pkg.ConfigMaps) == 0 && len(generatedConfigMaps) > 0 {
		pkg.ConfigMaps = generatedConfigMaps
	}
	if len(pkg.Files) == 0 && len(generatedFiles) > 0 {
		pkg.Files = generatedFiles
	}

	tmpl, err := ComponentConfigTemplateFromInput(ComponentConfigTemplateInput{
		Key:            pkg.Key,
		Name:           pkg.Name,
		Description:    pkg.Description,
		Framework:      pkg.Framework,
		BindingMode:    pkg.BindingMode,
		ComponentTypes: pkg.ComponentTypes,
		Syntax:         firstNonEmpty(pkg.Syntax, NativeComponentConfigTemplateSyntax),
		NativeConfigs:  pkg.NativeConfigs,
		Fields:         pkg.Fields,
		Env:            normalizeTemplatePackageObjects(pkg.Env),
		ConfigMaps:     normalizeTemplatePackageObjects(pkg.ConfigMaps),
		Secrets:        normalizeTemplatePackageObjects(pkg.Secrets),
		Files:          pkg.Files,
		Command:        pkg.Command,
		Args:           pkg.Args,
		Enabled:        pkg.Enabled,
	})
	if err != nil {
		return model.ComponentConfigTemplate{}, err
	}
	tmpl.SortOrder = pkg.SortOrder
	if tmpl.SortOrder == 0 {
		tmpl.SortOrder = 1000
	}
	return tmpl, nil
}

func ParseUploadedComponentConfigTemplateFile(filePath, fileName string, opts ComponentConfigTemplateUploadOptions) (model.ComponentConfigTemplate, error) {
	mode := normalizeComponentConfigTemplateUploadMode(opts.Mode)
	if strings.HasSuffix(strings.ToLower(strings.TrimSpace(fileName)), ".tar.gz") {
		if mode == "native" {
			return model.ComponentConfigTemplate{}, fmt.Errorf("ordinary config upload does not accept .tar.gz; upload files directly or use advanced mode")
		}
		return ParseComponentConfigTemplatePackageFile(filePath)
	}
	if mode == "advanced" {
		if tmpl, err := ParseComponentConfigTemplatePackageFile(filePath); err == nil {
			return tmpl, nil
		}
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return model.ComponentConfigTemplate{}, fmt.Errorf("read upload: %w", err)
	}
	if mode == "advanced" {
		return parseAdvancedComponentConfigTemplateJSON(data, fileName, opts)
	}
	return ParseNativeComponentConfigTemplateFiles([]NativeComponentConfigTemplateFile{{
		Name:    firstNonEmpty(strings.TrimSpace(opts.FileName), path.Base(fileName), defaultNativeTemplateFileName(opts.Framework)),
		Content: string(data),
	}}, opts)
}

func ParseNativeComponentConfigTemplateFiles(files []NativeComponentConfigTemplateFile, opts ComponentConfigTemplateUploadOptions) (model.ComponentConfigTemplate, error) {
	name := strings.TrimSpace(opts.Name)
	if name == "" && len(files) > 0 {
		name = strings.TrimSuffix(path.Base(files[0].Name), path.Ext(files[0].Name))
	}
	framework := firstNonEmpty(strings.TrimSpace(opts.Framework), "auto")
	componentTypes := firstStringArray(opts.ComponentTypes)

	fields := make([]map[string]interface{}, 0)
	fieldSeen := map[string]int{}
	configData := map[string]string{}
	secrets := make([]map[string]interface{}, 0)
	fileHints := make([]map[string]interface{}, 0)
	nativeConfigs := make([]map[string]interface{}, 0)
	for _, file := range files {
		fileName := firstNonEmpty(path.Base(strings.TrimSpace(file.Name)), defaultNativeTemplateFileName(framework))
		content := file.Content
		if strings.TrimSpace(fileName) == "" || content == "" {
			continue
		}
		parsed := ParseNativeComponentConfigTemplate(content, NativeComponentConfigTemplateOptions{Framework: framework, FileName: fileName})
		mergeNativeTemplateFields(&fields, fieldSeen, parsed.Fields)
		for _, configMap := range parsed.ConfigMaps {
			data, _ := configMap["data"].(map[string]string)
			for key, value := range data {
				configData[key] = value
			}
		}
		secrets = append(secrets, parsed.Secrets...)
		fileHints = append(fileHints, parsed.Files...)
		nativeConfigs = append(nativeConfigs, parsed.NativeConfigs...)
	}
	if len(nativeConfigs) == 0 {
		return model.ComponentConfigTemplate{}, fmt.Errorf("ordinary config upload must contain at least one non-empty config file")
	}
	return ComponentConfigTemplateFromInput(ComponentConfigTemplateInput{
		Key:            strings.TrimSpace(opts.Key),
		Name:           name,
		Description:    strings.TrimSpace(opts.Description),
		Framework:      framework,
		BindingMode:    firstNonEmpty(strings.TrimSpace(opts.BindingMode), "recommended"),
		ComponentTypes: componentTypes,
		Syntax:         NativeComponentConfigTemplateSyntax,
		NativeConfigs:  nativeConfigs,
		Fields:         fields,
		Env:            defaultNativeTemplateEnv(framework),
		ConfigMaps:     []map[string]interface{}{{"name": "{{configMapName}}", "data": configData}},
		Secrets:        secrets,
		Files:          fileHints,
		Command:        []string{},
		Args:           []string{},
	})
}

func parseAdvancedComponentConfigTemplateJSON(data []byte, fileName string, opts ComponentConfigTemplateUploadOptions) (model.ComponentConfigTemplate, error) {
	name := strings.TrimSpace(opts.Name)
	if name == "" {
		name = strings.TrimSuffix(path.Base(fileName), path.Ext(fileName))
	}
	framework := firstNonEmpty(strings.TrimSpace(opts.Framework), "auto")
	componentTypes := firstStringArray(opts.ComponentTypes)
	if looksLikeJSON(data, fileName) {
		var root map[string]interface{}
		if err := json.Unmarshal(data, &root); err != nil {
			return model.ComponentConfigTemplate{}, fmt.Errorf("parse template json: %w", err)
		}
		template := root
		if nested, ok := root["template"].(map[string]interface{}); ok {
			template = nested
		}
		encoded, _ := json.Marshal(template)
		var pkg componentConfigTemplatePackage
		if err := json.Unmarshal(encoded, &pkg); err != nil {
			return model.ComponentConfigTemplate{}, fmt.Errorf("decode template json: %w", err)
		}
		if schema, ok := root["schema"].(map[string]interface{}); ok {
			if fields, ok := schema["fields"].([]interface{}); ok && len(pkg.Fields) == 0 {
				fieldsJSON, _ := json.Marshal(fields)
				_ = json.Unmarshal(fieldsJSON, &pkg.Fields)
			}
		}
		tmpl, err := ComponentConfigTemplateFromInput(ComponentConfigTemplateInput{
			Key:            firstNonEmpty(pkg.Key, strings.TrimSpace(opts.Key)),
			Name:           firstNonEmpty(pkg.Name, name),
			Description:    firstNonEmpty(pkg.Description, strings.TrimSpace(opts.Description)),
			Framework:      firstNonEmpty(pkg.Framework, framework),
			BindingMode:    firstNonEmpty(pkg.BindingMode, strings.TrimSpace(opts.BindingMode), "recommended"),
			ComponentTypes: firstStringArray(pkg.ComponentTypes, componentTypes),
			Syntax:         firstNonEmpty(pkg.Syntax, ComponentConfigTemplateSyntax),
			NativeConfigs:  pkg.NativeConfigs,
			Fields:         pkg.Fields,
			Env:            normalizeTemplatePackageObjects(pkg.Env),
			ConfigMaps:     normalizeTemplatePackageObjects(pkg.ConfigMaps),
			Secrets:        normalizeTemplatePackageObjects(pkg.Secrets),
			Files:          pkg.Files,
			Command:        pkg.Command,
			Args:           pkg.Args,
			Enabled:        pkg.Enabled,
		})
		if err != nil {
			return model.ComponentConfigTemplate{}, err
		}
		tmpl.SortOrder = pkg.SortOrder
		if tmpl.SortOrder == 0 {
			tmpl.SortOrder = 1000
		}
		return tmpl, nil
	}
	return model.ComponentConfigTemplate{}, fmt.Errorf("advanced template upload must be JSON or .tar.gz package")
}

func normalizeComponentConfigTemplateUploadMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "advanced":
		return "advanced"
	default:
		return "native"
	}
}

func mergeNativeTemplateFields(fields *[]map[string]interface{}, seen map[string]int, items []map[string]interface{}) {
	for _, item := range items {
		key := templateMapString(item, "key")
		if key == "" {
			continue
		}
		if existingIndex, exists := seen[key]; exists {
			mergeNativeTemplateListItemFields((*fields)[existingIndex], item)
			continue
		}
		seen[key] = len(*fields)
		*fields = append(*fields, item)
	}
}

func mergeNativeTemplateListItemFields(existing map[string]interface{}, incoming map[string]interface{}) {
	existingItems, ok := existing["itemFields"].([]map[string]interface{})
	if !ok {
		return
	}
	incomingItems, ok := incoming["itemFields"].([]map[string]interface{})
	if !ok {
		return
	}
	for _, incomingItem := range incomingItems {
		key := templateMapString(incomingItem, "key")
		if key == "" || nativeTemplateFieldExists(existingItems, key) {
			continue
		}
		existingItems = append(existingItems, incomingItem)
	}
	existing["itemFields"] = existingItems
}

func UploadTemplateFileExt(fileName string) string {
	lower := strings.ToLower(strings.TrimSpace(fileName))
	if strings.HasSuffix(lower, ".tar.gz") {
		return ".tar.gz"
	}
	if ext := path.Ext(lower); ext != "" {
		return ext
	}
	return ".txt"
}

func looksLikeJSON(data []byte, fileName string) bool {
	if strings.EqualFold(path.Ext(fileName), ".json") {
		return true
	}
	trimmed := strings.TrimSpace(string(data))
	return strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[")
}

func firstStringArray(values ...[]string) []string {
	for _, value := range values {
		out := make([]string, 0, len(value))
		for _, item := range value {
			item = strings.TrimSpace(item)
			if item != "" {
				out = append(out, item)
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	return []string{}
}

func readComponentConfigTemplatePackage(filePath string) (map[string][]byte, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open template package: %w", err)
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		return nil, fmt.Errorf("template package must be .tar.gz: %w", err)
	}
	defer gz.Close()
	entries := map[string][]byte{}
	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read template package: %w", err)
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}
		name := cleanTemplatePackagePath(header.Name)
		if name == "" {
			continue
		}
		data, err := io.ReadAll(tr)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", name, err)
		}
		entries[name] = data
	}
	return entries, nil
}

func cleanTemplatePackagePath(value string) string {
	cleaned := path.Clean(strings.TrimSpace(strings.ReplaceAll(value, "\\", "/")))
	cleaned = strings.TrimPrefix(cleaned, "./")
	if cleaned == "." || strings.HasPrefix(cleaned, "../") || strings.HasPrefix(cleaned, "/") {
		return ""
	}
	return cleaned
}

func normalizeTemplatePackageObjects(items []map[string]interface{}) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		out = append(out, normalizeTemplatePackageValue(item).(map[string]interface{}))
	}
	return out
}

func normalizeTemplatePackageValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case string:
		return convertNativeTemplatePlaceholders(typed)
	case map[string]interface{}:
		out := map[string]interface{}{}
		for key, item := range typed {
			out[key] = normalizeTemplatePackageValue(item)
		}
		return out
	case []interface{}:
		out := make([]interface{}, 0, len(typed))
		for _, item := range typed {
			out = append(out, normalizeTemplatePackageValue(item))
		}
		return out
	default:
		return value
	}
}

var componentConfigTemplateKeyCleaner = regexp.MustCompile(`[^a-z0-9]+`)

func SlugifyComponentConfigTemplateKey(value string) string {
	key := strings.ToLower(strings.TrimSpace(value))
	key = componentConfigTemplateKeyCleaner.ReplaceAllString(key, "-")
	key = strings.Trim(key, "-")
	if len(key) > 56 {
		key = strings.Trim(key[:56], "-")
	}
	return key
}

func componentConfigTemplateKeySuffix(value string) string {
	if slug := SlugifyComponentConfigTemplateKey(value); slug != "" {
		return slug
	}
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(strings.TrimSpace(value)))
	return fmt.Sprintf("template-%08x", hash.Sum32())
}

func MustJSON(value interface{}) string {
	if value == nil {
		return "[]"
	}
	data, err := json.Marshal(value)
	if err != nil {
		return "[]"
	}
	return string(data)
}

func DecodeObjectArray(raw string) []map[string]interface{} {
	if strings.TrimSpace(raw) == "" {
		return []map[string]interface{}{}
	}
	var out []map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return []map[string]interface{}{}
	}
	if out == nil {
		return []map[string]interface{}{}
	}
	return out
}

func NormalizeComponentConfigTemplateFileHints(items []map[string]interface{}) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		key := templateMapString(item, "key")
		if key == "" {
			continue
		}
		normalized := map[string]interface{}{
			"key": key,
		}
		if name := templateMapString(item, "name"); name != "" {
			normalized["name"] = name
		}
		if configMapName := templateMapString(item, "configMapName"); configMapName != "" {
			normalized["configMapName"] = configMapName
		}
		if recommended := firstNonEmpty(templateMapString(item, "recommendedMountPath"), templateMapString(item, "mountPath")); recommended != "" {
			normalized["recommendedMountPath"] = recommended
		}
		if readOnly, exists := item["readOnly"]; exists {
			normalized["readOnly"] = readOnly != false
		} else {
			normalized["readOnly"] = true
		}
		out = append(out, normalized)
	}
	return out
}

func templateMapString(item map[string]interface{}, key string) string {
	if item == nil {
		return ""
	}
	value, exists := item[key]
	if !exists || value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func DecodeStringArray(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return []string{}
	}
	var out []string
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return []string{}
	}
	if out == nil {
		return []string{}
	}
	return out
}

type nativeBlockFrame struct {
	kind string
	key  string
}

func ParseNativeComponentConfigTemplate(source string, options NativeComponentConfigTemplateOptions) NativeParsedComponentConfigTemplate {
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
			addNativeTemplateField(&fields, fieldSeen, inferNativeTemplateField(token.key, token.label, token.defaultValue))
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
					itemFields = append(itemFields, inferNativeTemplateField(itemKey, token.label, token.defaultValue))
					fields[idx]["itemFields"] = itemFields
				}
				output.WriteString("[[paap:item." + itemKey + nativeTemplateDefaultOption(token.defaultValue) + "]]")
				continue
			}
			addNativeTemplateField(&fields, fieldSeen, inferNativeTemplateField(token.key, token.label, token.defaultValue))
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
	return NativeParsedComponentConfigTemplate{
		Fields: fields,
		Env:    defaultNativeTemplateEnv(framework),
		ConfigMaps: []map[string]interface{}{
			{"name": "{{configMapName}}", "data": map[string]string{fileName: output.String()}},
		},
		Secrets:       []map[string]interface{}{},
		Files:         []map[string]interface{}{file},
		NativeConfigs: []map[string]interface{}{{"name": fileName, "content": source}},
	}
}

func convertNativeTemplatePlaceholders(source string) string {
	var output strings.Builder
	stack := make([]nativeBlockFrame, 0)
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
			output.WriteString("[[paap:for " + token.key + "]]")
		case "IF":
			stack = append(stack, nativeBlockFrame{kind: "IF", key: token.key})
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
				itemKey := strings.TrimPrefix(token.key, "ITEM_")
				output.WriteString("[[paap:item." + itemKey + nativeTemplateDefaultOption(token.defaultValue) + "]]")
				continue
			}
			output.WriteString("[[paap:" + token.key + nativeTemplateDefaultOption(token.defaultValue) + "]]")
		}
	}
	return output.String()
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

func inferNativeTemplateField(key, label, defaultValue string) map[string]interface{} {
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
	if strings.Contains(key, "DIRECTIVES") || strings.Contains(key, "CONFIG_BLOCK") || strings.Contains(key, "CONTENT") || strings.HasSuffix(key, "BLOCK") {
		return "textarea"
	}
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
	if nativeTemplateHasAny(key, "MONGO", "MONGODB") && nativeTemplateHasAddressToken(key) {
		return "mongodb"
	}
	if strings.Contains(key, "EUREKA") && nativeTemplateHasAddressToken(key) {
		return "eureka"
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
