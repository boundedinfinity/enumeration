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
	"strings"

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
		"DO NOT EDIT",
		"",
		"Manual changes will be overwritten.",
		"",
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
		return errors.New("missing config path")
	}

	if !path.IsAbs(args.InputPath) {
		if absPath, err := filepath.Abs(args.InputPath); err != nil {
			return err
		} else {
			args.InputPath = absPath
		}
	}

	if !stringer.EndsWith(args.InputPath, ".enum.yaml") {
		return fmt.Errorf("%v must be a .enum.yaml file", args.InputPath)
	}

	if _, err := os.Stat(args.InputPath); err != nil {
		return fmt.Errorf("invalid config path %v: %w", args.InputPath, err)
	}

	return nil
}

func processEnum(args argsData, enum *enumer.EnumData) error {
	if bs, err := os.ReadFile(args.InputPath); err == nil {
		if err := yaml.Unmarshal(bs, &enum); err != nil {
			return fmt.Errorf("can't parse config path %v : %w", args.InputPath, err)
		}
	} else {
		return fmt.Errorf("can't load config path %v : %w", args.InputPath, err)
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

	for i := 0; i < len(enum.Values); i++ {
		value := enum.Values[i]

		if stringer.IsEmpty(value.Name) && stringer.IsEmpty(value.Serialized) {
			return fmt.Errorf("invalid values[%v] name or serialized value", i)
		} else if stringer.IsEmpty(value.Name) && stringer.IsDefined(value.Serialized) {
			value.Name = stringer.ReplaceNonLanguageCharacters(value.Serialized, " ", "_")
			value.Name = caser.Convert(value.Name, caser.CaseTypes.Phrase, caser.CaseTypes.Pascal)
		} else if stringer.IsDefined(value.Name) && stringer.IsEmpty(value.Serialized) {
			value.Serialized = stringer.RemoveNonLanguageCharacters(value.Name, "_")
			value.Serialized = caser.PascalToKebabLower(value.Serialized)
			value.Name = stringer.RemoveNonLanguageCharacters(value.Name, "_")
			value.Name = stringer.RemoveSpace(value.Name)
		} else if stringer.IsDefined(value.Name) && stringer.IsDefined(value.Serialized) {
			value.Name = stringer.RemoveNonLanguageCharacters(value.Name, "_")
			value.Name = stringer.RemoveSpace(value.Name)
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
			return fmt.Errorf("invalid header from path %v: %w", enum.HeaderFrom, err)
		}

		if bs, err := os.ReadFile(args.InputPath); err != nil {
			return fmt.Errorf("can't read header from path %v: %w", enum.HeaderFrom, err)
		} else {
			header := string(bs)
			enum.HeaderLines = stringer.Split(header, "\n")
		}
	}

	enum.Header = box(strings.Join(enum.HeaderLines, "\n"))

	if args.Overwrite {
		enum.Overwrite = true
	}

	return nil
}

func box(text string) string {
	lines := strings.Split(text, "\n")

	return asciibox.Box(
		lines,
		asciibox.BoxOptions{
			BoxWidth:      60,
			Alignment:     asciibox.Alignment_Middle,
			WrapCharacter: "/",
		},
	)
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
	pluralize := pluralize.NewClient()
	companionVar := pluralize.Plural(enum.Type)
	companionStruct := stringer.ToLowerFirst(companionVar)

	f := jen.NewFile(enum.Package)
	f.HeaderComment(enum.Header)

	f.Comment(box("Type")).Line()

	f.Type().Id(enum.Type).String().Line()

	f.Comment(box("Stringer implemenation")).Line()

	f.Func().Params(jen.Id("t").Id(enum.Type)).Id("String").Params().String().
		Block(jen.Return(jen.String().Params(jen.Id("t")))).
		Line()

	f.Comment(box("JSON marshal/unmarshal implemenation")).Line()

	f.Func().Params(jen.Id("t").Id(enum.Type)).
		Id("MarshalJSON").
		Params().Params(jen.Index().Byte(), jen.Error()).
		Block(jen.Return(
			jen.Qual("encoding/json", "Marshal").Params(jen.String().Parens(jen.Id("t"))),
		)).Line()

	f.Func().Params(jen.Id("t").Op("*").Id(enum.Type)).
		Id("UnmarshalJSON").
		Params(jen.Id("data").Index().Byte()).Params(jen.Error()).
		Block(
			jen.Var().Id("s").String().Line(),

			jen.If(
				jen.Err().Op(":=").Qual("encoding/json", "Unmarshal").
					Params(jen.Id("data"), jen.Op("&").Id("s")),
				jen.Err().Op("!=").Nil(),
			).Block(
				jen.Return(jen.Err()),
			).Line(),

			jen.Id("found").Op(",").Err().Op(":=").
				Id(companionVar).Dot("Parse").Call(jen.Id("s")).
				Line(),

			jen.If(jen.Err().Op("!=").Nil()).Block(jen.Return(jen.Err())).Line(),

			jen.Op("*").Id("t").Op("=").Id("found"),

			jen.Return(jen.Nil()),
		).Line()

	f.Comment(box("YAML marshal/unmarshal implemenation")).Line()

	f.Func().Params(jen.Id("t").Id(enum.Type)).Id("MarshalYAML").
		Params().Params(jen.Interface(), jen.Error()).
		Block(
			jen.Return(jen.String().Parens(jen.Id("t")).Op(",").Nil())).
		Line()

	f.Func().Params(jen.Id("t").Op("*").Id(enum.Type)).Id("UnmarshalYAML").Params(
		jen.Id("unmarshal").Func().Params(jen.Interface()).Error()).
		Error().
		Block(
			jen.Var().Id("s").String().Line(),

			jen.If(
				jen.Err().Op(":=").Id("unmarshal").
					Params(jen.Op("&").Id("s")),
				jen.Err().Op("!=").Nil(),
			).Block(
				jen.Return(jen.Err()),
			).Line(),

			jen.Id("found").Op(",").Err().Op(":=").
				Id(companionVar).Dot("Parse").Call(jen.Id("s")).
				Line(),

			jen.If(jen.Err().Op("!=").Nil()).Block(jen.Return(jen.Err())).Line(),

			jen.Op("*").Id("t").Op("=").Id("found"),

			jen.Return(jen.Nil()),
		).
		Line()

	f.Comment(box("XML marshal/unmarshal implemenation")).Line()

	f.Func().Params(jen.Id("t").Id(enum.Type)).Id("MarshalXML").Params(
		jen.Id("e").Op("*").Qual("encoding/xml", "Encoder"),
		jen.Id("start").Qual("encoding/xml", "StartElement"),
	).Error().Block(
		jen.Return(
			jen.Id("e").Dot("EncodeElement").Params(
				jen.String().Params(jen.Id("t")),
				jen.Id("start"),
			),
		),
	).Line()

	f.Func().Params(jen.Id("t").Op("*").Id(enum.Type)).Id("UnmarshalXML").Params(
		jen.Id("d").Op("*").Qual("encoding/xml", "Decoder"),
		jen.Id("start").Qual("encoding/xml", "StartElement"),
	).Error().Block(
		jen.Var().Id("s").String().Line(),

		jen.If(
			jen.Err().Op(":=").Id("d").Dot("DecodeElement").Params(
				jen.Op("&").Id("s"),
				jen.Op("&").Id("start"),
			),
			jen.Err().Op("!=").Nil(),
		).Block(
			jen.Return(jen.Err()),
		).Line(),

		jen.Id("found").Op(",").Err().Op(":=").
			Id(companionVar).Dot("Parse").Call(jen.Id("s")).
			Line(),

		jen.If(jen.Err().Op("!=").Nil()).Block(jen.Return(jen.Err())).Line(),

		jen.Op("*").Id("t").Op("=").Id("found"),

		jen.Return(jen.Nil()),
	).Line()

	f.Comment(box("SQL marshal/unmarshal implemenation")).Line()

	f.Func().Params(jen.Id("t").Id(enum.Type)).Id("Value").Params().Params(
		jen.Qual("database/sql/driver", "Value"),
		jen.Error(),
	).Block(
		jen.Return(
			jen.String().Params(jen.Id("t")),
			jen.Nil(),
		),
	).Line()

	f.Func().Params(jen.Id("t").Op("*").Id(enum.Type)).Id("Scan").Params(
		jen.Id("value").Interface(),
	).Error().Block(
		jen.If(
			jen.Id("value").Op("==").Nil().Block(
				jen.Return(jen.Id(companionVar).Dot("errf").Params(jen.Id("value"))),
			),
		).Line(),

		jen.Id("dv").Op(",").Err().Op(":=").Qual("database/sql/driver", "String").
			Dot("ConvertValue").Params(jen.Id("value")).Line(),

		jen.If(jen.Err().Op("!=").Nil()).Block(jen.Return(jen.Err())).Line(),

		jen.Id("s").Op(",").Id("ok").Op(":=").Id("dv").Assert(jen.String()).Line(),

		jen.If(jen.Op("!").Id("ok")).Block(
			jen.Return(jen.Id(companionVar).Dot("errf").Params(jen.Id("value"))),
		).Line(),

		jen.Id("found").Op(",").Err().Op(":=").
			Id(companionVar).Dot("Parse").Call(jen.Id("s")).
			Line(),

		jen.If(jen.Err().Op("!=").Nil()).Block(jen.Return(jen.Err())).Line(),

		jen.Op("*").Id("t").Op("=").Id("found"),

		jen.Return(jen.Nil()),
	).Line()

	f.Comment(box("Companion struct")).Line()

	f.Var().Id(companionVar).Op("=").Id(companionStruct).Values(jen.DictFunc(func(d jen.Dict) {
		d[jen.Id("Err")] = jen.Qual("fmt", "Errorf").Params(jen.Lit("invalid " + enum.Type))
		for _, value := range enum.Values {
			d[jen.Id(value.Name)] = jen.Id(enum.Type).Parens(jen.Lit(value.Serialized))
		}
	}))

	f.Type().Id(companionStruct).StructFunc(func(g *jen.Group) {
		g.Id("Err").Error()
		g.Id("errf").Func().Params(
			jen.Any(),
			jen.Op("...").Id(enum.Type),
		).Error()
		g.Id("parseMap").Map(jen.Id(enum.Type)).Index().String()

		for _, value := range enum.Values {
			g.Id(value.Name).Id(enum.Type)
		}
	})

	f.Func().Params(jen.Id("t").Id(companionStruct)).Id("Values").Params().Index().Id(enum.Type).Block(
		jen.Return(
			jen.Index().Id(enum.Type).ValuesFunc(func(g *jen.Group) {
				for _, value := range enum.Values {
					g.Line().Id(companionVar).Dot(value.Name)
				}
				g.Line()
			}),
		),
	).Line()

	f.Func().Params(jen.Id("t").Id(companionStruct)).Id("ParseFrom").Params(
		jen.Id("v").String(),
		jen.Id("items").Op("...").Id(enum.Type),
	).Params(
		jen.Id(enum.Type),
		jen.Error(),
	).Block(
		jen.Var().Id("found").Id(enum.Type),
		jen.Var().Id("ok").Bool().Line(),

		jen.For(
			jen.Id("_").Op(",").Id("item").Op(":=").Range().Id("items").Block(
				jen.Id("matchers").Op(",").Id("ok2").Op(":=").Id("t").Dot("parseMap").Index(jen.Id("item")).Line(),
				jen.If(jen.Op("!").Id("ok2")).Block(jen.Continue()).Line(),
				jen.For(
					jen.Id("_").Op(",").Id("matcher").Op(":=").Range().Id("matchers").Block(
						jen.If(jen.Id("v").Op("==").Id("matcher").Block(
							jen.Id("found").Op("=").Id("item"),
							jen.Id("ok").Op("=").True(),
							jen.Break(),
						)),
					),
				).Line(),
				jen.If(jen.Op("!").Id("ok").Block(
					jen.Return(
						jen.Id("found"),
						jen.Id("t").Dot("errf").Params(
							jen.Id("v"),
							jen.Id("items").Op("..."),
						),
					),
				)).Line(),
				jen.Return(jen.Id("found"), jen.Nil()),
			),
		).Line(),

		jen.Return(jen.Id("found").Op(",").Nil()),
	).Line()

	f.Func().Params(jen.Id("t").Id(companionStruct)).Id("Parse").Params(jen.Id("v").String()).Params(
		jen.Id(enum.Type).Op(",").Error(),
	).Block(
		jen.Return(jen.Id("t").Dot("ParseFrom").Params(
			jen.Id("v"),
			jen.Id("t").Dot("Values").Params().Op("..."),
		)),
	).Line()

	f.Func().Params(jen.Id("t").Id(companionStruct)).Id("IsFrom").Params(
		jen.Id("v").String(),
		jen.Id("items").Op("...").Id(enum.Type),
	).Bool().Block(
		jen.Id("_").Op(",").Err().Op(":=").Id("t").Dot("ParseFrom").Params(
			jen.Id("v"),
			jen.Id("items").Op("..."),
		),
		jen.Return(jen.Err().Op("==").Nil()),
	).Line()

	f.Func().Params(jen.Id("t").Id(companionStruct)).Id("Is").Params(jen.Id("v").String()).Bool().Block(
		jen.Return(jen.Id("t").Dot("IsFrom").Params(
			jen.Id("v"),
			jen.Id("t").Dot("Values").Params().Op("..."),
		)),
	).Line()

	f.Comment(box("Initialization")).Line()

	f.Func().Id("init").Params().BlockFunc(func(g *jen.Group) {
		g.Id(companionVar).Dot("parseMap").Op("=").Map(jen.Id(enum.Type)).Index().String().Values(jen.DictFunc(func(d jen.Dict) {
			for _, value := range enum.Values {
				if _, ok := d[jen.Lit(value.Name)]; !ok {
					d[jen.Id(companionVar).Dot(value.Name)] = jen.ValuesFunc(func(g2 *jen.Group) {
						g2.Lit(value.Serialized)
						g2.Lit(value.Name)
						for _, from := range value.ParseFrom {
							g2.Lit(from)
						}
					})
				}
			}
		})).Line()

		g.Id(companionVar).Dot("errf").Op("=").Func().Params(
			jen.Id("v").Any(),
			jen.Id("items").Op("...").Id(enum.Type),
		).Error().Block(
			jen.Var().Id("xs").Index().String().Line(),
			jen.For(
				jen.Op("_").Op(",").Id("item").Op(":=").Range().Id("items"),
			).Block(
				jen.If(
					jen.Id("x").Op(",").Id("ok").Op(":=").Id(companionVar).Dot("parseMap").Index(jen.Id("item")),
					jen.Id("ok"),
				).Block(
					jen.Id("xs").Op("=").Append(jen.Id("xs"), jen.Id("x").Op("...")),
				),
			).Line(),
			jen.Return(jen.Qual("fmt", "Errorf").Params(
				jen.Line().Lit("%w: %v is not one of %s"),
				jen.Line().Id(companionVar).Dot("Err"),
				jen.Line().Id("v"),
				jen.Line().Qual("strings", "Join").Params(jen.Id("xs"), jen.Lit(",")),
				jen.Line(),
			)),
		)
	}).Line()

	content := fmt.Sprintf("%#v", f)
	return []byte(content), nil
}
