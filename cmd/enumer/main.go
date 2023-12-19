package main

import (
	_ "embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/boundedinfinity/asciibox"
	"github.com/boundedinfinity/enumer"
	"github.com/boundedinfinity/go-commoner/idiomatic/caser"
	"github.com/boundedinfinity/go-commoner/idiomatic/extentioner"
	"github.com/boundedinfinity/go-commoner/idiomatic/pather"
	"github.com/boundedinfinity/go-commoner/idiomatic/stringer"
	"github.com/dave/jennifer/jen"
	"github.com/gertd/go-pluralize"
	"gopkg.in/yaml.v2"
)

//go:embed settings.json
var vscodeSettingsContext string

const (
	FilePermissions = 0644
)

var (
	jsonSchemaName     = "enum.schema.json"
	vscodeSettingsName = "settings.json"

	Header = []string{
		"===== DO NOT EDIT =====",
		"Manual changes will be overwritten.",
		"Generated by github.com/boundedinfinity/enumer",
	}
)

type argsData struct {
	InputPath  string
	SkipFormat bool
	Debug      bool
	VsCode     string
	Serialize  string
	Overwrite  bool
}

func handleErr(err error) {
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}

func main() {
	var args argsData

	if err := processArgs(&args); err != nil {
		handleErr(err)
	}

	if args.VsCode != "" {
		if err := processJsonSchema(args); err != nil {
			handleErr(err)
		}
	} else {
		var enum enumer.EnumData

		if err := processEnum(args, &enum); err != nil {
			handleErr(err)
		}

		bs, err := processTemplate(enum)

		if err != nil {
			handleErr(err)
		}

		if err := processWrite(enum, bs); err != nil {
			handleErr(err)
		}
	}
}

func generateJsonSchema() string {
	m := map[string]any{
		"$schema": "http://json-schema.org/draft-07/schema",
		"title":   "Bounded Infinity enumeration tool",
		"type":    "object",
		"version": "1.0.19",
		"properties": map[string]any{
			"type": map[string]any{
				"type": "string",
			},
			"package": map[string]any{
				"type": "string",
			},
			"output-path": map[string]any{
				"type": "string",
			},
			"desc": map[string]any{
				"type": "string",
			},
			"header": map[string]any{
				"type": "string",
			},
			"header-from": map[string]any{
				"type": "string",
			},
			"serialize": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"type": map[string]any{
						"type": "string",
						"enum": caser.ConverterCombinations(),
					},
					"value": map[string]any{
						"type": "string",
						"enum": caser.ConverterCombinations(),
					},
				},
			},
			"skip-format": map[string]any{
				"type": "boolean",
			},
			"debug": map[string]any{
				"type": "boolean",
			},
			"values": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"name": map[string]any{
							"type": "string",
						},
						"serialized": map[string]any{
							"type": "string",
						},
					},
				},
			},
		},
	}

	bs, err := json.MarshalIndent(m, "", "    ")

	if err != nil {
		panic(err)
	}

	return string(bs)
}

func processJsonSchema(args argsData) error {
	projectSettingsDir := pather.Join(args.VsCode, ".vscode")

	if _, err := pather.Dirs.EnsureErr(projectSettingsDir); err != nil {
		return err
	}

	projectSettingsPath := pather.Join(projectSettingsDir, vscodeSettingsName)

	if pather.Paths.Exists(projectSettingsPath) {
		name := fmt.Sprintf("enumer-schema-%v", vscodeSettingsName)
		projectSettingsPath = pather.Join(projectSettingsDir, name)

		if err := os.WriteFile(projectSettingsPath, []byte(vscodeSettingsContext), FilePermissions); err != nil {
			return err
		}

		fmt.Printf("Add the contents of the %v file to your %v file.",
			name,
			vscodeSettingsName,
		)
	} else {
		if err := os.WriteFile(projectSettingsPath, []byte(vscodeSettingsContext), FilePermissions); err != nil {
			return err
		}
	}

	projectEnumsPath := pather.Join(projectSettingsDir, jsonSchemaName)

	if err := os.WriteFile(projectEnumsPath, []byte(generateJsonSchema()), FilePermissions); err != nil {
		return err
	}

	return nil
}

func processArgs(args *argsData) error {
	flag.StringVar(&args.InputPath, "config", "", "The input file used for the enum being generated.")
	flag.BoolVar(&args.SkipFormat, "skip-format", false, "Skip source formatting.")
	flag.BoolVar(&args.Debug, "debug", false, "Enabled debugging.")
	flag.StringVar(&args.VsCode, "vscode", "", "Path to project to configure the Visual Studio Code JSON Schema file.")
	flag.Parse()

	if args.VsCode != "" {
		return nil
	}

	if args.InputPath == "" {
		return errors.New("Missing config path.  The input file used for the enum being generated.")
	}

	if !path.IsAbs(args.InputPath) {
		if absPath, err := filepath.Abs(args.InputPath); err != nil {
			return err
		} else {
			args.InputPath = absPath
		}
	}

	if !stringer.EndsWith(args.InputPath, ".enum.yaml") {
		return fmt.Errorf("%v must be a .enum.yaml file\n", args.InputPath)
	}

	if _, err := os.Stat(args.InputPath); err != nil {
		return fmt.Errorf("Invalid config path %v: %w", args.InputPath, err)
	}

	return nil
}

func processEnum(args argsData, enum *enumer.EnumData) error {
	if bs, err := os.ReadFile(args.InputPath); err == nil {
		if err := yaml.Unmarshal(bs, &enum); err != nil {
			return fmt.Errorf("Can't parse config path %v : %w.", args.InputPath, err)
		}
	} else {
		return fmt.Errorf("Can't load config path %v : %w.", args.InputPath, err)
	}

	if args.SkipFormat {
		enum.SkipFormat = args.SkipFormat
	}

	if args.Debug {
		enum.Debug = args.Debug
	}

	enum.InputPath = args.InputPath

	if enum.OutputPath == "" {
		enum.OutputPath = extentioner.Swap(enum.InputPath, ".yaml", ".go")
	}

	if enum.Package == "" {
		enum.Package = enum.OutputPath
		enum.Package = pather.Paths.Dir(enum.Package)
		enum.Package = pather.Paths.Base(enum.Package)
		enum.Package = stringer.ReplaceInList(enum.Package, []string{"-", " "}, "_")
	}

	var typeConverter func(string) string

	if enum.Serialize.Type != "" {
		if c, err := caser.Converter[string](enum.Serialize.Type); err != nil {
			return err
		} else {
			typeConverter = c
		}
	} else {
		typeConverter = caser.PhraseToPascal[string]
	}

	if enum.Type == "" {
		enum.Type = enum.OutputPath
		enum.Type = pather.Paths.Base(enum.Type)
		enum.Type = extentioner.Strip(enum.Type)
		enum.Type = extentioner.Strip(enum.Type)
		enum.Type = caser.KebabToPascal(enum.Type)
	}

	if enum.Struct == "" {
		enum.Struct = enum.Type
		enum.Struct = pluralize.NewClient().Plural(enum.Struct)
	}

	var valueConverter func(string) string
	passthrough := func(s string) string {
		return s
	}

	if !stringer.IsEmpty(enum.Serialize.Value) {
		if c, err := caser.Converter[string](enum.Serialize.Value); err != nil {
			return err
		} else {
			valueConverter = c
		}
	} else {
		valueConverter = passthrough
	}

	for i := 0; i < len(enum.Values); i++ {
		value := enum.Values[i]

		if stringer.IsEmpty(value.Name) && stringer.IsEmpty(value.Serialized) {
			return fmt.Errorf("Invalid values[%v] name or serialized value", i)
		} else if stringer.IsEmpty(value.Name) && !stringer.IsEmpty(value.Serialized) {
			value.Name = valueConverter(value.Serialized)
			value.Name = stringer.RemoveSymbols(value.Name)
			value.Name = stringer.RemoveSpace(value.Name)
		} else if !stringer.IsEmpty(value.Name) && stringer.IsEmpty(value.Serialized) {
			value.Serialized = typeConverter(value.Name)
		}

		enum.Values[i] = value
	}

	if enum.Header == "" && enum.HeaderFrom == "" {
		enum.HeaderLines = Header
	}

	if enum.Header != "" {
		enum.HeaderLines = stringer.Split(enum.Header, "\n")
	}

	if enum.HeaderFrom != "" {
		if _, err := os.Stat(enum.HeaderFrom); err != nil {
			return fmt.Errorf("Invalid header from path %v: %w", enum.HeaderFrom, err)
		}

		if bs, err := os.ReadFile(args.InputPath); err != nil {
			return fmt.Errorf("Can't read header from path %v: %w", enum.HeaderFrom, err)
		} else {
			header := string(bs)
			enum.HeaderLines = stringer.Split(header, "\n")
		}
	}

	enum.Header = asciibox.Box(
		enum.HeaderLines,
		asciibox.BoxOptions{Alignment: asciibox.Alignment_Left},
	)

	if args.Overwrite {
		enum.Overwrite = true
	}

	return nil
}

func processWrite(enum enumer.EnumData, bs []byte) error {
	if pather.Paths.Exists(enum.OutputPath) {
		if enum.Overwrite {
			if _, err := pather.Paths.RemoveErr(enum.OutputPath); err != nil {
				return err
			}
		} else {
			return nil
		}
	}

	dir := pather.Paths.Dir(enum.OutputPath)
	err := os.MkdirAll(dir, FilePermissions)

	if err != nil {
		return err
	}

	if _, err := pather.Paths.RemoveErr(enum.OutputPath); err != nil {
		return err
	}

	if err := os.WriteFile(enum.OutputPath, bs, FilePermissions); err != nil {
		return err
	}

	return nil
}

func processTemplate(enum enumer.EnumData) ([]byte, error) {
	enumerPkg := "github.com/boundedinfinity/enumer"
	pluralize := pluralize.NewClient()
	companionVar := pluralize.Plural(enum.Type)
	companionStruct := stringer.ToLowerFirst(companionVar)

	f := jen.NewFile(enum.Package)
	f.Comment(enum.Header).Line()

	f.Type().Id(enum.Type).String().Line()

	f.Func().Params(jen.Id("t").Id(enum.Type)).Id("String").Params().String().
		Block(jen.Return(jen.String().Params(jen.Id("t")))).
		Line()

	f.Comment(`// /////////////////////////////////////////////////////////////////
    //  JSON serializatoin
    // /////////////////////////////////////////////////////////////////
    `)

	f.Func().Params(jen.Id("t").Id(enum.Type)).
		Id("MarshalJSON").
		Params().Params(jen.Index().Byte(), jen.Error()).
		Block(jen.Return(jen.Qual(enumerPkg, "MarshalJSON").Params(jen.Id("t")))).
		Line()

	f.Func().Params(jen.Id("t").Op("*").Id(enum.Type)).
		Id("UnmarshalJSON").
		Params(jen.Id("data").Index().Byte()).Params(jen.Error()).
		Block(jen.Return(jen.Qual(enumerPkg, "UnmarshalJSON").Params(
			jen.Id("data"),
			jen.Id("t"),
			jen.Id(companionVar).Dot("Parse"),
		))).
		Line()

	f.Comment(`// /////////////////////////////////////////////////////////////////
    //  YAML serializatoin
    // /////////////////////////////////////////////////////////////////
    `)

	f.Func().Params(jen.Id("t").Id(enum.Type)).Id("MarshalYAML").
		Params().Params(jen.Interface(), jen.Error()).
		Block(jen.Return(jen.Qual(enumerPkg, "MarshalYAML").Params(jen.Id("t")))).
		Line()

	f.Func().Params(jen.Id("t").Op("*").Id(enum.Type)).Id("UnmarshalYAML").
		Params(jen.Id("unmarshal").Func().Params(jen.Interface()).Error()).
		Error().
		Block(jen.Return(jen.Qual(enumerPkg, "UnmarshalYAML").Params(
			jen.Id("unmarshal"),
			jen.Id("t"),
			jen.Id(companionVar).Dot("Parse"),
		))).
		Line()

	f.Comment(`// /////////////////////////////////////////////////////////////////
    //  XML serializatoin
    // /////////////////////////////////////////////////////////////////
    `)

	f.Func().Params(jen.Id("t").Id(enum.Type)).Id("MarshalXML").
		Params(
			jen.Id("e").Op("*").Qual("encoding/xml", "Encoder"),
			jen.Id("start").Qual("encoding/xml", "StartElement"),
		).Error().
		Block(jen.Return(jen.Qual(enumerPkg, "MarshalXML").Params(
			jen.Id("t"),
			jen.Id("e"),
			jen.Id("start"),
		))).
		Line()

	f.Func().Params(jen.Id("t").Op("*").Id(enum.Type)).Id("UnmarshalXML").
		Params(
			jen.Id("d").Op("*").Qual("encoding/xml", "Decoder"),
			jen.Id("start").Qual("encoding/xml", "StartElement"),
		).Error().
		Block(jen.Return(jen.Qual(enumerPkg, "UnmarshalXML").Params(
			jen.Id("t"),
			jen.Id(companionVar).Dot("Parse"),
			jen.Id("d"),
			jen.Id("start"),
		))).
		Line()

	f.Comment(`// /////////////////////////////////////////////////////////////////
    //  SQL serializatoin
    // /////////////////////////////////////////////////////////////////
    `)

	f.Func().Params(jen.Id("t").Id(enum.Type)).Id("Value").
		Params().Params(jen.Qual("database/sql/driver", "Value"), jen.Error()).
		Block(jen.Return(jen.Qual(enumerPkg, "Value").Params(jen.Id("t")))).
		Line()

	f.Func().Params(jen.Id("t").Op("*").Id(enum.Type)).Id("Scan").
		Params(
			jen.Id("value").Interface(),
		).
		Error().
		Block(jen.Return(jen.Qual(enumerPkg, "Scan").Params(
			jen.Id("value"),
			jen.Id("t"),
			jen.Id(companionVar).Dot("Parse"),
		))).
		Line()

	f.Comment(`// /////////////////////////////////////////////////////////////////
    //  Companion
    // /////////////////////////////////////////////////////////////////
    `)

	f.Type().Id(companionStruct).StructFunc(func(g *jen.Group) {
		for _, value := range enum.Values {
			g.Id(value.Name).Id(enum.Type)
		}

		g.Id("Values").Index().Id(enum.Type)
		g.Id("Err").Error()
	})

	f.Var().Id(companionVar).Op("=").Id(companionStruct).Block(jen.DictFunc(func(d jen.Dict) {
		d[jen.Id("Err")] = jen.Qual("fmt", "Errorf").Params(jen.Lit("invalid " + enum.Type))
		for _, value := range enum.Values {
			d[jen.Id(value.Name)] = jen.Id(enum.Type).Parens(jen.Lit(value.Serialized))
		}
	}))

	f.Func().Id("init").Params().BlockFunc(func(g *jen.Group) {
		g.Id(companionVar).Dot("Values").Op("=").Index().Id(enum.Type).ValuesFunc(func(g *jen.Group) {
			for _, value := range enum.Values {
				g.Line().Id(companionVar).Dot(value.Name)
			}
			g.Line()
		})
	}).Line()

	f.Func().Params(jen.Id("t").Id(companionStruct)).Id("newErr").Params(
		jen.Id("a").Any(),
		jen.Id("values").Op("...").Id(enum.Type),
	).Error().Block(jen.Return(
		jen.Qual("fmt", "Errorf").Params(
			jen.Line().Lit("invalid %w value '%v'. Must be one of %v"),
			jen.Line().Id(companionVar).Dot("Err"),
			jen.Line().Id("a"),
			jen.Line().Qual(enumerPkg, "Join").Params(
				jen.Id("values"),
				jen.Lit(", "),
			),
		),
	)).Line()

	f.Func().Params(jen.Id("t").Id(companionStruct)).Id("ParseFrom").Params(
		jen.Id("v").String(),
		jen.Id("values").Op("...").Id(enum.Type),
	).Params(
		jen.Id(enum.Type),
		jen.Error(),
	).BlockFunc(func(g *jen.Group) {
		g.Var().Id("found").Id(enum.Type)
		g.Var().Id("ok").Bool().Line()
		g.For().Id("_").Op(",").Id("value").Op(":=").Range().Id("values").BlockFunc(func(g *jen.Group) {
			g.If(
				jen.Qual(enumerPkg, "IsEq").Types(
					jen.String(),
					jen.Id(enum.Type),
				).Params(jen.Id("v")).Params(jen.Id("value")).Block(
					jen.Id("found").Op("=").Id("value"),
					jen.Id("ok").Op("=").True(),
					jen.Break(),
				),
			)
		}).Line()

		g.If(jen.Op("!").Id("ok")).Block(jen.Return(
			jen.Id("found"),
			jen.Id("t").Dot("newErr").Params(
				jen.Id("v"),
				jen.Id("values").Op("..."),
			),
		)).Line()

		g.Return(
			jen.Id("found"),
			jen.Nil(),
		)
	}).Line()

	f.Func().Params(jen.Id("t").Id(companionStruct)).Id("Parse").Params(
		jen.Id("v").String(),
	).Params(
		jen.Id(enum.Type),
		jen.Error(),
	).BlockFunc(func(g *jen.Group) {
		g.Return().Id("t").Dot("ParseFrom").Params(
			jen.Id("v"),
			jen.Id(companionVar).Dot("Values").Op("..."),
		)
	}).Line()

	f.Func().Params(jen.Id("t").Id(companionStruct)).Id("IsFrom").Params(
		jen.Id("v").String(),
		jen.Id("values").Op("...").Id(enum.Type),
	).Bool().Block(
		jen.For().Id("_").Op(",").Id("value").Op(":=").Range().Id("values").Block(
			jen.If(
				jen.Qual(enumerPkg, "IsEq").
					Types(jen.String(), jen.Id(enum.Type)).
					Params(jen.Id("v")).
					Params(jen.Id("value")).Block(
					jen.Return().True(),
				),
			),
		),
		jen.Return().False(),
	).Line()

	f.Func().Params(
		jen.Id("t").Id(companionStruct),
	).Id("Is").Params(
		jen.Id("v").String(),
	).Bool().Block(
		jen.Return().Id("t").Dot("IsFrom").Params(
			jen.Id("v"),
			jen.Id(companionVar).Dot("Values").Op("..."),
		))

	content := fmt.Sprintf("%#v", f)
	return []byte(content), nil
}
