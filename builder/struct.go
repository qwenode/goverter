package builder

import (
	"fmt"
	"go/types"
	"strings"

	"github.com/dave/jennifer/jen"
	"github.com/jmattheis/goverter/config"
	"github.com/jmattheis/goverter/method"
	"github.com/jmattheis/goverter/xtype"
)

// Struct handles struct types.
type Struct struct{}

// Matches returns true, if the builder can create handle the given types.
func (*Struct) Matches(_ *MethodContext, source, target *xtype.Type) bool {
	return source.Struct && target.Struct
}

// Build creates conversion source code for the given source and target type.
func (s *Struct) Build(gen Generator, ctx *MethodContext, sourceID *xtype.JenID, source, target *xtype.Type, errPath ErrorPath) ([]jen.Code, *xtype.JenID, *Error) {
	// Optimization for golang sets
	if !source.Named && !target.Named && source.StructType.NumFields() == 0 && target.StructType.NumFields() == 0 {
		return nil, sourceID, nil
	}
	return BuildByAssign(s, gen, ctx, sourceID, source, target, errPath)
}

func (s *Struct) Assign(gen Generator, ctx *MethodContext, assignTo *AssignTo, sourceID *xtype.JenID, source, target *xtype.Type, errPath ErrorPath) ([]jen.Code, *Error) {
	additionalFieldSources, err := parseAutoMap(ctx, source)
	if err != nil {
		return nil, err
	}

	stmt := []jen.Code{}

	definedFields := ctx.DefinedFields(target)
	usedSourceID := false
	for i := 0; i < target.StructType.NumFields(); i++ {
		targetField := target.StructType.Field(i)
		delete(definedFields, targetField.Name())

		fieldMapping := ctx.Field(target, targetField.Name())

		if fieldMapping.Ignore {
			continue
		}
		if !targetField.Exported() && ctx.Conf.IgnoreUnexported {
			continue
		}

		if !xtype.Accessible(targetField, ctx.OutputPackagePath) {
			cause := unexportedStructError(targetField.Name(), source.String, target.String)
			return nil, NewError(cause).Lift(&Path{
				Prefix:     ".",
				SourceID:   "???",
				TargetID:   targetField.Name(),
				TargetType: targetField.Type().String(),
			})
		}

		targetFieldType := xtype.TypeOf(targetField.Type())
		targetFieldPath := errPath.Field(targetField.Name())

		if fieldMapping.Function == nil {
			usedSourceID = true
			nextID, nextSource, mapStmt, lift, skip, err := mapField(gen, ctx, targetField, sourceID, source, target, additionalFieldSources, targetFieldPath)
			if skip {
				continue
			}
			if err != nil {
				return nil, err
			}
			stmt = append(stmt, mapStmt...)

			fieldStmt, err := gen.Assign(ctx, AssignOf(assignTo.Stmt.Clone().Dot(targetField.Name())), nextID, nextSource, targetFieldType, targetFieldPath)
			if err != nil {
				return nil, err.Lift(lift...)
			}
			if shouldCheckAgainstZero(ctx, nextSource, targetFieldType, assignTo.Update, false) {
				stmt = append(stmt, jen.If(nextID.Code.Clone().Op("!=").Add(xtype.ZeroValue(nextSource.T))).Block(fieldStmt...))
			} else {
				stmt = append(stmt, fieldStmt...)
			}
		} else {
			def := fieldMapping.Function

			sourceLift := []*Path{}
			var functionCallSourceID *xtype.JenID
			var functionCallSourceType *xtype.Type
			if def.Source != nil {
				usedSourceID = true
				nextID, nextSource, mapStmt, mapLift, _, err := mapField(gen, ctx, targetField, sourceID, source, target, additionalFieldSources, targetFieldPath)
				if err != nil {
					return nil, err
				}
				sourceLift = mapLift
				stmt = append(stmt, mapStmt...)

				if fieldMapping.Source == "." && sourceID.ParentPointer != nil &&
					def.Source.AssignableTo(source.AsPointer()) {
					functionCallSourceID = sourceID.ParentPointer
					functionCallSourceType = source.AsPointer()
				} else {
					functionCallSourceID = nextID
					functionCallSourceType = nextSource
				}
			} else {
				sourceLift = append(sourceLift, &Path{
					Prefix:     ".",
					TargetID:   targetField.Name(),
					TargetType: targetFieldType.String,
				})
			}

			callStmt, callReturnID, err := gen.CallMethod(ctx, fieldMapping.Function, functionCallSourceID, functionCallSourceType, targetFieldType, targetFieldPath)
			if err != nil {
				return nil, err.Lift(sourceLift...)
			}
			callStmt = append(callStmt, assignTo.Stmt.Clone().Dot(targetField.Name()).Op("=").Add(callReturnID.Code))

			if shouldCheckAgainstZero(ctx, functionCallSourceType, targetFieldType, assignTo.Update, true) {
				stmt = append(stmt, jen.If(functionCallSourceID.Code.Clone().Op("!=").Add(xtype.ZeroValue(functionCallSourceType.T))).Block(callStmt...))
			} else {
				stmt = append(stmt, callStmt...)
			}
		}
	}
	if !usedSourceID {
		stmt = append(stmt, jen.Id("_").Op("=").Add(sourceID.Code.Clone()))
	}

	for name := range definedFields {
		return nil, NewError(fmt.Sprintf("Field %q does not exist.\nRemove or adjust field settings referencing this field.", name)).Lift(&Path{
			Prefix:     ".",
			TargetID:   name,
			TargetType: "???",
		})
	}

	return stmt, nil
}

func shouldCheckAgainstZero(ctx *MethodContext, s, t *xtype.Type, isUpdate, call bool) bool {
	switch {
	case !ctx.Conf.UpdateTarget && !isUpdate:
		return false
	case s.Struct && ctx.Conf.IgnoreStructZeroValueField:
		return true
	case s.Basic && ctx.Conf.IgnoreBasicZeroValueField:
		return true
	case ctx.Conf.IgnoreNillableZeroValueField:
		if s.Chan || s.Map || s.Func || s.Signature || s.Interface {
			return true
		}
		if call || (ctx.Conf.SkipCopySameType && types.Identical(s.T, t.T)) {
			return (s.List && !s.ListFixed) || s.Pointer
		}
		return false
	default:
		return false
	}
}

func mapField(
	gen Generator,
	ctx *MethodContext,
	targetField *types.Var,
	sourceID *xtype.JenID,
	source, target *xtype.Type,
	additionalFieldSources []xtype.FieldSources,
	errPath ErrorPath,
) (*xtype.JenID, *xtype.Type, []jen.Code, []*Path, bool, *Error) {
	lift := []*Path{}
	def := ctx.Field(target, targetField.Name())
	
	// 处理 argmap 映射
	if def.ArgIndex > 0 {
		// 检查参数索引是否有效
		if def.ArgIndex > len(ctx.Conf.RawArgs) {
			return nil, nil, nil, nil, false, NewError(fmt.Sprintf("argmap index $%d is out of range, method has %d arguments", def.ArgIndex, len(ctx.Conf.RawArgs))).Lift(&Path{
				Prefix:     ".",
				SourceID:   fmt.Sprintf("$%d", def.ArgIndex),
				SourceType: "goverter:argmap",
				TargetID:   targetField.Name(),
				TargetType: targetField.Type().String(),
			})
		}
		
		// 获取对应的参数
		arg := ctx.Conf.RawArgs[def.ArgIndex-1] // 参数索引从1开始，数组索引从0开始
		argType := arg.Type
		argName := arg.Name
		
		// 创建参数访问的代码
		argID := xtype.VariableID(jen.Id(argName))
		
		lift = append(lift, &Path{
			Prefix:     ".",
			SourceID:   fmt.Sprintf("$%d (%s)", def.ArgIndex, argName),
			SourceType: fmt.Sprintf("goverter:argmap %s", argType.String),
			TargetID:   targetField.Name(),
			TargetType: targetField.Type().String(),
		})
		
		return argID, argType, nil, lift, false, nil
	}
	
	pathString := def.Source
	if pathString == "." {
		lift = append(lift, &Path{
			Prefix:     ".",
			SourceID:   " ",
			SourceType: "goverter:map . " + targetField.Name(),
			TargetID:   targetField.Name(),
			TargetType: targetField.Type().String(),
		})
		return sourceID, source, nil, lift, false, nil
	}

	var path []string
	if pathString == "" {
		sourceMatch, err := xtype.FindField(targetField.Name(), ctx.Conf.MatchIgnoreCase, source, additionalFieldSources)
		if err != nil {
			cause := fmt.Sprintf("Cannot match the target field with the source entry: %s.", err.Error())
			skip := false
			if ctx.Conf.IgnoreMissing {
				_, skip = err.(*xtype.NoMatchError)
			}
			return nil, nil, nil, nil, skip, NewError(cause).Lift(&Path{
				Prefix:     ".",
				SourceID:   "???",
				TargetID:   targetField.Name(),
				TargetType: targetField.Type().String(),
			})
		}

		path = sourceMatch.Path
	} else {
		path = strings.Split(pathString, ".")
	}

	var condition *jen.Statement

	nextIDCode := sourceID.Code
	nextSource := source

	for i := 0; i < len(path); i++ {
		if nextSource.Pointer {
			addCondition := nextIDCode.Clone().Op("!=").Nil()
			if condition == nil {
				condition = addCondition
			} else {
				condition = condition.Clone().Op("&&").Add(addCondition)
			}
			nextSource = nextSource.PointerInner
		}
		if !nextSource.Struct {
			cause := fmt.Sprintf("Cannot access '%s' on %s.", path[i], nextSource.T)
			return nil, nil, nil, nil, false, NewError(cause).Lift(&Path{
				Prefix:     ".",
				SourceID:   path[i],
				SourceType: "???",
			}).Lift(lift...)
		}
		sourceMatch, err := xtype.FindExactField(nextSource, path[i])
		if err == nil {
			nextSource = sourceMatch.Type
			nextIDCode = nextIDCode.Clone().Dot(sourceMatch.Name)
			liftPath := &Path{
				Prefix:     ".",
				SourceID:   sourceMatch.Name,
				SourceType: nextSource.String,
			}

			if i == len(path)-1 {
				liftPath.TargetID = targetField.Name()
				liftPath.TargetType = targetField.Type().String()
			}
			lift = append(lift, liftPath)
			continue
		}

		cause := fmt.Sprintf("Cannot find the mapped field on the source entry: %s.", err.Error())
		return nil, nil, []jen.Code{}, nil, false, NewError(cause).Lift(&Path{
			Prefix:     ".",
			SourceID:   path[i],
			SourceType: "???",
		}).Lift(lift...)
	}

	returnID := xtype.VariableID(nextIDCode)
	innerStmt := []jen.Code{}
	if nextSource.Func {
		def, err := method.Parse(nextSource.FuncType, &method.ParseOpts{
			Converter:         nil,
			OutputPackagePath: ctx.OutputPackagePath,
			ErrorPrefix:       "Error parsing struct method",
			Params:            method.ParamsNone,
			ContextMatch:      config.StructMethodContextRegex,
			CustomCall:        nextIDCode,
		}, method.EmptyLocalOpts)
		if err != nil {
			return nil, nil, nil, nil, false, NewError(err.Error()).Lift(lift...)
		}

		methodCallInner, callID, callErr := gen.CallMethod(ctx, def, nil, nil, def.Target, errPath)
		if callErr != nil {
			return nil, nil, nil, nil, false, callErr.Lift(lift...)
		}
		innerStmt = methodCallInner
		nextSource = def.Target
		returnID = callID
		lift = append(lift, &Path{
			Prefix:     "(",
			SourceID:   ")",
			SourceType: def.Target.String,
		})
	}

	if condition != nil && !nextSource.Pointer {
		lift[len(lift)-1].SourceType = fmt.Sprintf("*%s (It is a pointer because the nested property in the goverter:map was a pointer)",
			lift[len(lift)-1].SourceType)
	}

	stmt := []jen.Code{}
	if condition != nil {
		pointerNext := nextSource
		if !nextSource.Pointer {
			pointerNext = nextSource.AsPointer()
		}
		tempName := ctx.Name(pointerNext.ID())
		stmt = append(stmt, jen.Var().Id(tempName).Add(pointerNext.TypeAsJen()))

		if nextSource.Pointer {
			innerStmt = append(innerStmt, jen.Id(tempName).Op("=").Add(returnID.Code))
		} else {
			pstmt, pointerID := returnID.Pointer(nextSource, ctx.Name)
			innerStmt = append(innerStmt, pstmt...)
			innerStmt = append(innerStmt, jen.Id(tempName).Op("=").Add(pointerID.Code))
		}

		stmt = append(stmt, jen.If(condition).Block(innerStmt...))
		nextSource = pointerNext
		returnID = xtype.VariableID(jen.Id(tempName))
	} else {
		stmt = append(stmt, innerStmt...)
	}

	return returnID, nextSource, stmt, lift, false, nil
}

func parseAutoMap(ctx *MethodContext, source *xtype.Type) ([]xtype.FieldSources, *Error) {
	fieldSources := []xtype.FieldSources{}
	for _, field := range ctx.Conf.AutoMap {
		innerSource := source
		lift := []*Path{}
		path := strings.Split(field, ".")
		for _, part := range path {
			field, err := xtype.FindExactField(innerSource, part)
			if err != nil {
				return nil, NewError(err.Error()).Lift(&Path{
					Prefix:     ".",
					SourceID:   part,
					SourceType: "goverter:autoMap",
				}).Lift(lift...)
			}
			lift = append(lift, &Path{
				Prefix:     ".",
				SourceID:   field.Name,
				SourceType: field.Type.String,
			})
			innerSource = field.Type

			switch {
			case innerSource.Pointer && innerSource.PointerInner.Struct:
				innerSource = xtype.TypeOf(innerSource.PointerInner.StructType)
			case innerSource.Struct:
				// ok
			default:
				return nil, NewError(fmt.Sprintf("%s is not a struct or struct pointer", part)).Lift(lift...)
			}
		}

		fieldSources = append(fieldSources, xtype.FieldSources{Path: path, Type: innerSource})
	}
	return fieldSources, nil
}

func unexportedStructError(targetField, sourceType, targetType string) string {
	return fmt.Sprintf(`Cannot set value for unexported field "%s".

See https://goverter.jmattheis.de/guide/unexported-field`, targetField)
}
