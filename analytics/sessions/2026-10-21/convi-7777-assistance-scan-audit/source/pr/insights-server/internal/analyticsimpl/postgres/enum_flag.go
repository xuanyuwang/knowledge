package postgres

import analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"

type EnumFlag int64

type FlaggableEnum interface {
	analyticspb.QAAttributeType
}

func CreateEnumFlag[T FlaggableEnum](enumValues []T) EnumFlag {
	var flag EnumFlag
	for _, enumValue := range enumValues {
		flag |= 1 << enumValue
	}
	return flag
}

func HasEnumValue[T FlaggableEnum](flag EnumFlag, enumValue T) bool {
	return flag&(1<<enumValue) > 0
}
