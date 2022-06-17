package fhserver

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-playground/validator/v10"
	jsoniter "github.com/json-iterator/go"
	"github.com/savsgio/gotils/strconv"
	errs "github.com/spacetab-io/errors-go"
	pkgErr "github.com/spacetab-io/http-go/errors"
	"github.com/valyala/fasthttp"
)

type (
	Response struct {
		Error *errs.ErrorObject `json:"error,omitempty"`
		Data  interface{}       `json:"data,omitempty"`
	}
	validationRule   string
	errorPattern     string
	validationErrors map[validationRule]errorPattern
)

var (
	json = jsoniter.ConfigCompatibleWithStandardLibrary

	defaultLang            = "ru"
	CommonValidationErrors = map[string]validationErrors{
		"ru": {
			"ek":       "Ошибка валидации для свойства `%s` с правилом `%s`",
			"required": "Свойство `%s` обязательно для заполнения",
			"gt":       "Свойство `%s` должно содержать более `%s` элементов",
		},
	}
)

// JSON makes common response in json.
func JSON(ctx *fasthttp.RequestCtx, response interface{}) {
	ctx.SetContentType("application/json")

	lang := getLang(ctx)

	obj, code := data(ctx, response, lang)

	res, err := json.Marshal(&obj)
	// We are now no longer need the buffer so we pool it.
	if err != nil {
		ctx.SetStatusCode(http.StatusInternalServerError)
		ctx.SetBody(strconv.S2B(fmt.Sprintf(`{"error":"%v"}`, err.Error())))
	}

	if ctx.Response.Header.StatusCode() == http.StatusOK {
		ctx.SetStatusCode(code)
	}

	ctx.SetBody(res)
}

func data(ctx *fasthttp.RequestCtx, item interface{}, lang string) (result Response, code int) {
	errType := errs.ErrorTypeError

	var obj Response

	switch item := item.(type) {
	case []error:
		errObj := errs.ErrorObject{}
		code = http.StatusInternalServerError

		msgs := make([]string, 0)
		for _, e := range item {
			msgs = append(msgs, e.Error())
		}

		errObj.Message = strings.Join(msgs, "; ")
		errObj.Type = &errType
		obj.Error = &errObj
	case validator.ValidationErrors:
		code = http.StatusUnprocessableEntity
		errObj := errs.ErrorObject{}

		errObj.Message = "validation error"
		errObj.Validation = makeErrorsSlice(item, lang)
		obj.Error = &errObj
	case error:
		errObj := errs.ErrorObject{}
		code, errObj.Message = getErrCode(item)
		obj.Error = &errObj
	case map[string]error:
		errObj := errs.ErrorObject{}

		msgs := make(map[string]string)
		for k, e := range item {
			msgs[k] = e.Error()
		}

		errObj.Message = msgs
		obj.Error = &errObj
	case string:
		if ctx.Response.Header.StatusCode() >= http.StatusBadRequest {
			errObj := errs.ErrorObject{}
			code = ctx.Response.Header.StatusCode()
			errObj.Message = item
			obj.Error = &errObj
		} else {
			obj.Data = item
			code = http.StatusOK
		}
	case []byte:
		if ctx.Response.Header.StatusCode() >= http.StatusBadRequest {
			errObj := errs.ErrorObject{}
			code = ctx.Response.Header.StatusCode()
			errObj.Message = strconv.B2S(item)
			obj.Error = &errObj
		} else {
			obj.Data = strconv.B2S(item)
			code = http.StatusOK
		}

	default:
		code = http.StatusOK
		obj.Data = item
	}

	return obj, code
}

func (ve errorPattern) string() string {
	return string(ve)
}

func getLang(c *fasthttp.RequestCtx) string {
	lang := c.Request.Header.Peek("Content-Language")
	if len(lang) == 0 {
		return defaultLang
	}

	return string(lang)
}

// validationErrors Формирование массива ошибок.
func makeErrorsSlice(err validator.ValidationErrors, lang string) map[errs.FieldName][]errs.ValidationError {
	ve := make(map[errs.FieldName][]errs.ValidationError)

	for _, e := range err {
		field := getFieldName(e.Namespace(), e.Field())

		if _, ok := ve[field]; !ok {
			ve[field] = make([]errs.ValidationError, 0)
		}

		ve[field] = append(
			ve[field],
			getErrMessage(validationRule(e.ActualTag()), field, e.Param(), lang),
		)
	}

	return ve
}

func getFieldName(namespace, field string) errs.FieldName {
	namespace = strings.ReplaceAll(namespace, "]", "")
	namespace = strings.ReplaceAll(namespace, "[", ".")
	namespaceSlice := strings.Split(namespace, ".")
	fieldName := field

	if len(namespaceSlice) > 2 { //nolint: gomnd // жёстко проверяется на наличие более чем 2х элементов слайса
		fieldName = strings.Join([]string{strings.Join(namespaceSlice[1:len(namespaceSlice)-1], "."), field}, ".")
	}

	return errs.FieldName(fieldName)
}

func getErrMessage(errorType validationRule, field errs.FieldName, param, lang string) errs.ValidationError {
	errKey := errorType

	_, ok := CommonValidationErrors[lang][errorType]
	if !ok {
		errKey = "ek"
	}

	if param != "" && errKey == "ek" {
		return errs.ValidationError(fmt.Sprintf(CommonValidationErrors[lang][errKey].string(), field, errorType))
	}

	return errs.ValidationError(fmt.Sprintf(CommonValidationErrors[lang][errKey].string(), field))
}

func getErrCode(err error) (errCode int, msg string) {
	msg = err.Error()

	switch {
	case errors.Is(err, pkgErr.ErrNotFound):
		errCode = http.StatusNotFound
	case errors.Is(err, pkgErr.ErrNoMethod):
		errCode = http.StatusMethodNotAllowed
	case errors.Is(err, pkgErr.ErrServerError), errors.Is(err, sql.ErrConnDone), errors.Is(err, sql.ErrTxDone):
		errCode = http.StatusInternalServerError
	case errors.Is(err, pkgErr.ErrRecordNotFound):
		errCode = http.StatusNotFound
	case errors.Is(err, pkgErr.ErrConflict):
		errCode = http.StatusConflict
	case errors.Is(err, sql.ErrNoRows):
		errCode = http.StatusNotFound
		msg = pkgErr.ErrRecordNotFound.Error()
	default:
		errCode = http.StatusBadRequest
	}

	return
}
