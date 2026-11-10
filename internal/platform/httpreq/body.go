package httpreq

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"io"
	"mime/multipart"
	"net/url"
	"strings"
)

const (
	TypeJSON     = "json"
	TypeForm     = "form"
	TypeFormData = "formdata"
	TypeXML      = "xml"
	TypeText     = "text"

	maxBody = 64 * 1024
)

// Payload 是按请求体类型编码后的内容和 Content-Type。
type Payload struct {
	Body        io.Reader
	ContentType string
}

// Normalize 校验并规范化请求体。allowEmpty 为 true 时，未选择类型且没有内容会原样留空；
// 未选择类型但已有内容时按文本处理。POST 应传 false，必须明确选择类型。
func Normalize(bodyType, body string, allowEmpty bool) (string, string, error) {
	if strings.ContainsRune(body, 0) {
		return "", "", errors.New("请求体不能包含空字符")
	}
	bodyType = strings.ToLower(strings.TrimSpace(bodyType))
	if bodyType == "" {
		if strings.TrimSpace(body) == "" {
			if allowEmpty {
				return "", "", nil
			}
			return "", "", errors.New("请选择请求体类型")
		}
		if !allowEmpty {
			return "", "", errors.New("请选择请求体类型")
		}
		bodyType = TypeText
	}
	var err error
	switch bodyType {
	case TypeJSON:
		body, err = normalizeJSON(body)
	case TypeForm, TypeFormData:
		body, err = normalizePairs(body)
	case TypeXML:
		body, err = normalizeXML(body)
	case TypeText:
		body, err = normalizeText(body)
	default:
		return "", "", errors.New("请求体类型仅支持 JSON、Form、FormData、XML、Text")
	}
	if err != nil {
		return "", "", err
	}
	if len(body) > maxBody {
		return "", "", errors.New("请求体不能超过 64KB")
	}
	return bodyType, body, nil
}

// Encode 把已规范化的请求体编码成实际发送的内容。
func Encode(bodyType, body string) (Payload, error) {
	bodyType = strings.ToLower(strings.TrimSpace(bodyType))
	switch bodyType {
	case "":
		if body == "" {
			return Payload{}, nil
		}
		return Payload{Body: strings.NewReader(body), ContentType: "text/plain; charset=utf-8"}, nil
	case TypeJSON:
		return Payload{Body: strings.NewReader(body), ContentType: "application/json"}, nil
	case TypeForm:
		return Payload{Body: strings.NewReader(body), ContentType: "application/x-www-form-urlencoded"}, nil
	case TypeFormData:
		raw, contentType, err := multipartBody(body)
		if err != nil {
			return Payload{}, err
		}
		return Payload{Body: bytes.NewReader(raw), ContentType: contentType}, nil
	case TypeXML:
		return Payload{Body: strings.NewReader(body), ContentType: "application/xml; charset=utf-8"}, nil
	case TypeText:
		return Payload{Body: strings.NewReader(body), ContentType: "text/plain; charset=utf-8"}, nil
	default:
		return Payload{}, errors.New("请求体类型仅支持 JSON、Form、FormData、XML、Text")
	}
}

func normalizeJSON(body string) (string, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return "", errors.New("JSON 内容不能为空")
	}
	if !json.Valid([]byte(body)) {
		return "", errors.New("JSON 格式不正确")
	}
	return body, nil
}

func normalizeXML(body string) (string, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return "", errors.New("XML 内容不能为空")
	}
	dec := xml.NewDecoder(strings.NewReader(body))
	for {
		_, err := dec.Token()
		if err == io.EOF {
			return body, nil
		}
		if err != nil {
			return "", errors.New("XML 格式不正确")
		}
	}
}

func normalizeText(body string) (string, error) {
	return body, nil
}

func normalizePairs(body string) (string, error) {
	pairs, err := pairsFrom(body)
	if err != nil {
		return "", err
	}
	if len(pairs) == 0 {
		return "", nil
	}
	values := url.Values{}
	for _, pair := range pairs {
		values.Add(pair[0], pair[1])
	}
	return values.Encode(), nil
}

func pairsFrom(body string) ([][2]string, error) {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	body = strings.TrimSpace(body)
	if body == "" {
		return nil, nil
	}
	if strings.Contains(body, "\n") {
		var pairs [][2]string
		for _, line := range strings.Split(body, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			key, val, ok := strings.Cut(line, "=")
			key = strings.TrimSpace(key)
			if !ok || key == "" {
				return nil, errors.New("表单需要写成 key=value，每行一对")
			}
			pairs = append(pairs, [2]string{key, strings.TrimSpace(val)})
		}
		return pairs, nil
	}
	var pairs [][2]string
	for _, part := range strings.Split(body, "&") {
		if part == "" {
			continue
		}
		rawKey, rawVal, ok := strings.Cut(part, "=")
		if !ok {
			return nil, errors.New("表单需要写成 key=value")
		}
		key, err := url.QueryUnescape(strings.TrimSpace(rawKey))
		if err != nil || key == "" {
			return nil, errors.New("表单需要写成 key=value")
		}
		val, err := url.QueryUnescape(rawVal)
		if err != nil {
			return nil, errors.New("表单需要写成 key=value")
		}
		pairs = append(pairs, [2]string{key, val})
	}
	if len(pairs) == 0 {
		return nil, errors.New("表单需要写成 key=value")
	}
	return pairs, nil
}

func multipartBody(body string) ([]byte, string, error) {
	pairs, err := pairsFrom(body)
	if err != nil {
		return nil, "", err
	}
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	for _, pair := range pairs {
		if err := writer.WriteField(pair[0], pair[1]); err != nil {
			return nil, "", err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), writer.FormDataContentType(), nil
}
