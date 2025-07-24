package config

import (
	"fmt"
	"go/types"
	"regexp"
	"strings"

	"github.com/jmattheis/goverter/config/parse"
	"github.com/jmattheis/goverter/method"
)

const (
	configMap     = "map"
	configDefault = "default"
)

var StructMethodContextRegex = regexp.MustCompile(".*")

type Method struct {
	*method.Definition
	Common

	Constructor *method.Definition
	AutoMap     []string
	Fields      map[string]*FieldMapping
	EnumMapping *EnumMapping

	RawFieldSettings []string

	Location    string
	updateParam string
	localOpts   method.LocalOpts
}

type FieldMapping struct {
	Source   string
	Function *method.Definition
	Ignore   bool
	ArgIndex int // 用于argmap，表示从第几个参数获取值，0表示不使用argmap
}

func (m *Method) Field(targetName string) *FieldMapping {
	target, ok := m.Fields[targetName]
	if !ok {
		target = &FieldMapping{}
		m.Fields[targetName] = target
	}
	return target
}

func parseMethods(ctx *context, rawConverter *RawConverter, c *Converter) error {
	if c.typ != nil {
		interf := c.typ.Underlying().(*types.Interface)
		for i := 0; i < interf.NumMethods(); i++ {
			fun := interf.Method(i)
			def, err := parseMethod(ctx, c, fun, rawConverter.Methods[fun.Name()])
			if err != nil {
				return err
			}
			c.Methods = append(c.Methods, def)
		}
		return nil
	}
	for name, lines := range rawConverter.Methods {
		_, fn, err := ctx.Loader.GetOneRaw(c.Package, name)
		if err != nil {
			return err
		}
		def, err := parseMethod(ctx, c, fn, lines)
		if err != nil {
			return err
		}
		c.Methods = append(c.Methods, def)
	}
	return nil
}

func parseMethod(ctx *context, c *Converter, obj types.Object, rawMethod RawLines) (*Method, error) {
	m := &Method{
		Common:      c.Common,
		Fields:      map[string]*FieldMapping{},
		Location:    rawMethod.Location,
		EnumMapping: &EnumMapping{Map: map[string]string{}},
		localOpts:   method.LocalOpts{Context: map[string]bool{}},
	}

	for _, value := range rawMethod.Lines {
		if err := parseMethodLine(ctx, c, m, value); err != nil {
			return m, formatLineError(rawMethod, obj.String(), value, err)
		}
	}

	// 检查是否有使用argmap的字段，如果有则允许多源参数
	hasArgMap := false
	for _, field := range m.Fields {
		if field.ArgIndex > 0 {
			hasArgMap = true
			break
		}
	}

	def, err := method.Parse(obj, &method.ParseOpts{
		ErrorPrefix:       "error parsing converter method",
		Location:          rawMethod.Location,
		Converter:         nil,
		OutputPackagePath: c.OutputPackagePath,
		Params:            method.ParamsRequired,
		ParamsMultiSource: hasArgMap,
		ContextMatch:      m.ArgContextRegex,
		Generated:         true,
		UpdateParam:       m.updateParam,
	}, m.localOpts)

	m.Definition = def

	return m, err
}

func parseMethodLine(ctx *context, c *Converter, m *Method, value string) (err error) {
	cmd, rest := parse.Command(value)
	fieldSetting := false
	switch cmd {
	case configMap:
		fieldSetting = true
		var source, target, custom string
		source, target, custom, err = parseMethodMap(rest)
		if err != nil {
			return err
		}
		f := m.Field(target)
		f.Source = source

		if custom != "" {
			opts := &method.ParseOpts{
				ErrorPrefix:       "error parsing type",
				OutputPackagePath: c.OutputPackagePath,
				Converter:         c.typeForMethod(),
				Params:            method.ParamsOptional,
				AllowTypeParams:   true,
				ContextMatch:      m.ArgContextRegex,
			}
			f.Function, err = ctx.Loader.GetOne(c.Package, custom, opts)
		}
	case "ignore":
		fieldSetting = true
		fields := strings.Fields(rest)
		for _, f := range fields {
			m.Field(f).Ignore = true
		}
	case "update":
		m.updateParam, err = parse.String(rest)
	case "context":
		var key string
		key, err = parse.String(rest)
		m.localOpts.Context[key] = true
	case "enum:map":
		fields := strings.Fields(rest)
		if len(fields) != 2 {
			return fmt.Errorf("invalid fields")
		}

		if IsEnumAction(fields[1]) {
			err = validateEnumAction(fields[1])
		}

		m.EnumMapping.Map[fields[0]] = fields[1]
	case "enum:transform":
		fields := strings.SplitN(rest, " ", 2)

		config := ""
		if len(fields) == 2 {
			config = fields[1]
		}

		var t ConfiguredTransformer
		t, err = parseTransformer(ctx, fields[0], config)
		m.EnumMapping.Transformers = append(m.EnumMapping.Transformers, t)
	case "autoMap":
		fieldSetting = true
		var s string
		s, err = parse.String(rest)
		m.AutoMap = append(m.AutoMap, strings.TrimSpace(s))
	case "argmap":
		fieldSetting = true
		var argIndex int
		var target string
		argIndex, target, err = parseMethodArgMap(rest)
		if err != nil {
			return err
		}
		f := m.Field(target)
		f.ArgIndex = argIndex
	case configDefault:
		opts := &method.ParseOpts{
			ErrorPrefix:       "error parsing type",
			OutputPackagePath: c.OutputPackagePath,
			Converter:         c.typeForMethod(),
			Params:            method.ParamsOptional,
			AllowTypeParams:   true,
			ContextMatch:      m.ArgContextRegex,
		}
		m.Constructor, err = ctx.Loader.GetOne(c.Package, rest, opts)
	default:
		fieldSetting, err = parseCommon(&m.Common, cmd, rest)
	}
	if fieldSetting {
		m.RawFieldSettings = append(m.RawFieldSettings, value)
	}
	return err
}

func parseMethodMap(remaining string) (source, target, custom string, err error) {
	parts := strings.SplitN(remaining, "|", 2)
	if len(parts) == 2 {
		custom = strings.TrimSpace(parts[1])
	}

	fields := strings.Fields(parts[0])
	switch len(fields) {
	case 1:
		target = fields[0]
	case 2:
		source = fields[0]
		target = fields[1]
	case 0:
		err = fmt.Errorf("missing target field")
	default:
		err = fmt.Errorf("too many fields expected at most 2 fields got %d: %s", len(fields), remaining)
	}
	if err == nil && strings.ContainsRune(target, '.') {
		err = fmt.Errorf("the mapping target %q must be a field name but was a path.\nDots \".\" are not allowed.", target)
	}
	return source, target, custom, err
}

func parseMethodArgMap(remaining string) (argIndex int, target string, err error) {
	fields := strings.Fields(remaining)
	if len(fields) != 2 {
		err = fmt.Errorf("argmap requires exactly 2 fields: $<index> <target_field>, got %d: %s", len(fields), remaining)
		return
	}

	argStr := fields[0]
	target = fields[1]

	// 检查参数格式是否为 $数字
	if !strings.HasPrefix(argStr, "$") {
		err = fmt.Errorf("argument index must start with '$', got: %s", argStr)
		return
	}

	// 解析数字部分
	indexStr := argStr[1:]
	if indexStr == "" {
		err = fmt.Errorf("missing argument index after '$'")
		return
	}

	// 简单的数字解析
	argIndex = 0
	for _, r := range indexStr {
		if r < '0' || r > '9' {
			err = fmt.Errorf("invalid argument index: %s, must be a number", indexStr)
			return
		}
		argIndex = argIndex*10 + int(r-'0')
	}

	if argIndex < 1 {
		err = fmt.Errorf("argument index must be >= 1, got: %d", argIndex)
		return
	}

	// 检查目标字段名是否有效
	if strings.ContainsRune(target, '.') {
		err = fmt.Errorf("the mapping target %q must be a field name but was a path.\nDots \".\" are not allowed.", target)
		return
	}

	return argIndex, target, nil
}
