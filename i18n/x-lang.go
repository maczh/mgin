package i18n

import (
	"errors"
	"github.com/maczh/mgin/client"
)

const (
	SERVICE_X_LANGUAGE          = "x-lang"
	URI_LIST_STRINGS_BY_APPNAME = "/lang/string/list"
	URI_GET_APP_STRINGS_VERSION = "/lang/string/app/version"
)

func GetAppXLangVersion(appName string) (string, error) {
	params := map[string]string{
		"appName": appName,
	}
	rs := client.CallT[map[string]string](SERVICE_X_LANGUAGE, URI_GET_APP_STRINGS_VERSION, &client.Options{
		Method:   "POST",
		Protocol: client.CONTENT_TYPE_FORM,
		Group:    "DEFAULT_GROUP",
		Data:     params,
		Retry:    false,
	})
	return rs.Data["version"], nil
}

func GetAppXLangStringsAll(appName string) (map[string]string, error) {
	params := map[string]string{
		"appName": appName,
	}
	rs := client.CallT[map[string]string](SERVICE_X_LANGUAGE, URI_LIST_STRINGS_BY_APPNAME, &client.Options{
		Method:   "POST",
		Protocol: client.CONTENT_TYPE_FORM,
		Group:    "DEFAULT_GROUP",
		Data:     params,
		Retry:    false,
	})
	if rs.Status != 1 {
		return nil, errors.New(rs.Msg)
	}
	return rs.Data, nil
}
