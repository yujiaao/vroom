package main

import (
	"context"
	"net/url"
	"strconv"

	"github.com/rs/zerolog/log"

	"github.com/getsentry/vroom/internal/snubautil"
)

var (
	profileFilterFields = map[string]string{
		"device_classification":  "device_classification",
		"device_locale":          "device_locale",
		"device_manufacturer":    "device_manufacturer",
		"device_model":           "device_model",
		"device_os_build_number": "device_os_build_number",
		"device_os_name":         "device_os_name",
		"device_os_version":      "device_os_version",
		"environment":            "environment",
		"platform":               "platform",
		"transaction_id":         "transaction_id",
		"transaction_name":       "transaction_name",
	}

	profileQueryFilterMakers = []func(url.Values) ([]string, error){
		snubautil.MakeProjectsFilter,
		func(params url.Values) ([]string, error) {
			return snubautil.MakeTimeRangeFilter("received", params)
		},
		func(params url.Values) ([]string, error) {
			return snubautil.MakeFieldsFilter(profileFilterFields, params)
		},
		snubautil.MakeAndroidApiLevelFilter,
		snubautil.MakeVersionNameAndCodeFilter,
	}
)

type (
	VersionBuild struct {
		Name string
		Code string
	}
)

func (e *environment) profilesQueryBuilderFromRequest(ctx context.Context, p url.Values) (snubautil.QueryBuilder, error) {
	sqb, err := e.snuba.NewQuery(ctx, "profiles")
	if err != nil {
		return snubautil.QueryBuilder{}, err
	}
	sqb.WhereConditions = make([]string, 0, 5)

	for _, makeFilter := range profileQueryFilterMakers {
		conditions, err := makeFilter(p)
		if err != nil {
			return snubautil.QueryBuilder{}, err
		}
		sqb.WhereConditions = append(sqb.WhereConditions, conditions...)
	}

	if v := p.Get("limit"); v != "" {
		limit, err := strconv.Atoi(v)
		if err != nil {
			log.Err(err).Str("limit", v).Msg("can't parse limit value")
			return snubautil.QueryBuilder{}, err
		}
		sqb.Limit = limit
	}

	if v := p.Get("offset"); v != "" {
		offset, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			log.Err(err).Str("offset", v).Msg("can't parse offset value")
			return snubautil.QueryBuilder{}, err
		}
		sqb.Offset = offset
	}

	return sqb, nil
}

func snubaProfileToProfileResult(profile snubautil.Profile) ProfileResult {
	return ProfileResult{
		AndroidAPILevel:      profile.AndroidAPILevel,
		DeviceClassification: profile.DeviceClassification,
		DeviceLocale:         profile.DeviceLocale,
		DeviceManufacturer:   profile.DeviceManufacturer,
		DeviceModel:          profile.DeviceModel,
		DeviceOsBuildNumber:  profile.DeviceOsBuildNumber,
		DeviceOsName:         profile.DeviceOsName,
		DeviceOsVersion:      profile.DeviceOsVersion,
		ID:                   profile.ProfileID,
		ProjectID:            strconv.FormatUint(profile.ProjectID, 10),
		Timestamp:            profile.Received.Unix(),
		TraceDurationMs:      float64(profile.DurationNs) / 1_000_000,
		TransactionID:        profile.TransactionID,
		TransactionName:      profile.TransactionName,
		VersionCode:          profile.VersionCode,
		VersionName:          profile.VersionName,
	}
}
