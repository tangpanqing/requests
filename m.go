package requests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Response struct {
	StatusCode int
	Text       string
	Content    []byte

	Header     http.Header
	Cookies    []*http.Cookie
	CookiesUrl string

	RespBody io.ReadCloser
}

type Args struct {
	//Dictionary, list of tuples or bytes to send in the query string for the Request.
	Params any
	//Dictionary, list of tuples, bytes, or file-like object to send in the body of the Request.
	Data any
	//A JSON serializable Python object to send in the body of the Request.
	Json any
	//Dictionary of HTTP Headers to send with the Request.
	Headers map[string]string
	//Dictionary or CookieJar object to send with the Request.
	Cookies []*http.Cookie
	//Dictionary of 'name': file-like-objects (or {'name': file-tuple}) for multipart encoding upload. file-tuple can be a 2-tuple ('filename', fileobj), 3-tuple ('filename', fileobj, 'content_type') or a 4-tuple ('filename', fileobj, 'content_type', custom_headers), where 'content-type' is a string defining the content type of the given file and custom_headers a dict-like object containing additional headers to add for the file.
	Files map[string]string
	//Auth tuple to enable Basic/Digest/Custom HTTP Auth.
	Auth string
	//How many seconds to wait for the server to send data before giving up, as a float, or a (connect timeout, read timeout) tuple.
	Timeout float32
	//Boolean. Enable/disable GET/OPTIONS/POST/PUT/PATCH/DELETE/HEAD redirection. Defaults to True.
	AllowRedirects bool
	//Dictionary mapping protocol to the URL of the proxy.
	Proxies string
	//Either a boolean, in which case it controls whether we verify the server’s TLS certificate, or a string, in which case it must be a path to a CA bundle to use. Defaults to True.
	Verify bool
	//if False, the response content will be immediately downloaded.
	Stream bool
	//if String, path to ssl client cert file (.pem). If Tuple, (‘cert’, ‘key’) pair.
	Cert any
}

func (r *Response) Json() (map[string]any, error) {
	pp := make(map[string]any)
	err := json.Unmarshal([]byte(r.Text), &pp)
	if err != nil {
		return pp, err
	}
	return pp, nil
}

func (r *Response) GetHeader(name string) string {
	return r.Header.Get(name)
}

func (r *Response) GetCookie(name string) string {
	for i := 0; i < len(r.Cookies); i++ {
		if r.Cookies[i].Name == name {
			return r.Cookies[i].Value
		}
	}

	return ""
}

func Get(url string, args ...Args) Response {
	return Request("GET", url, args...)
}

func Options(url string, args ...Args) Response {
	return Request("OPTIONS", url, args...)
}

func Head(url string, args ...Args) Response {
	return Request("HEAD", url, args...)
}

func Post(url string, args ...Args) Response {
	return Request("POST", url, args...)
}

func Put(url string, args ...Args) Response {
	return Request("PUT", url, args...)
}

func Patch(url string, args ...Args) Response {
	return Request("PATCH", url, args...)
}

func Delete(url string, args ...Args) Response {
	return Request("DELETE", url, args...)
}

func Request(method string, urlStr string, args ...Args) Response {
	//客户端，响应体，请求
	client := &http.Client{}
	client.Timeout = time.Duration(time.Duration(1).Seconds())
	client.Transport = http.DefaultTransport

	response := Response{}

	var bodyReq io.Reader

	if len(args) > 0 {
		arg := args[0]

		//设置params
		paramStr := getParamStr(arg.Params)
		if paramStr != "" {
			urlStr += "?" + paramStr
		}

		//设置data
		data := getData(arg.Data, arg.Headers)

		if data != "" {
			bodyReq = bytes.NewBuffer([]byte(data))
		}

		//不允许转跳
		if !arg.AllowRedirects {
			client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			}
		}
	} else {
		//不允许转跳
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}

	req, err := http.NewRequest(method, urlStr, bodyReq)
	if err != nil {
		fmt.Println(err)
	}

	//如果有参数
	if len(args) > 0 {
		arg := args[0]

		//设置headers
		for k, v := range arg.Headers {
			req.Header.Set(k, v)
		}

		//设置cookie
		if len(arg.Cookies) > 0 {
			for _, v := range arg.Cookies {
				v := v
				req.AddCookie(v)
			}
		}

		//设置代理
		if arg.Proxies != "" {
			proxyUrl, _ := url.Parse(arg.Proxies)
			client.Transport = &http.Transport{
				Proxy: http.ProxyURL(proxyUrl),
			}
		}
	}

	//发送请求
	resp, err := client.Do(req)
	//异常响应
	if err != nil {
		response.Text = err.Error()
		if resp != nil {
			response.StatusCode = resp.StatusCode
		}
		return response
	}
	//defer resp.Body.Close()

	response.StatusCode = resp.StatusCode
	response.Header = resp.Header
	response.Cookies = resp.Cookies()

	if "text/event-stream" == resp.Header.Get("Content-type") {
		response.RespBody = resp.Body
	} else {
		body, _ := ioutil.ReadAll(resp.Body)
		response.Content = body
		response.Text = string(body)
		for i := 0; i < len(response.Cookies); i++ {
			response.Cookies[i].Domain = resp.Request.Host
		}
	}
	return response
}

func getParamStr(params any) string {
	str := ""
	switch value := params.(type) {
	case string:
		str = params.(string)
	case map[string]any:
		mm := make([]string, 0)
		for k, v := range value {
			mm = append(mm, k+"="+fmt.Sprintf("%v", v))
		}
		str = strings.Join(mm, "&")
	default:

	}
	return str
}

func getData(data any, header map[string]string) string {
	if header["Content-Type"] == "application/json" {
		jsonData, _ := json.Marshal(&data)
		return string(jsonData)
	}

	str := ""
	switch value := data.(type) {
	case string:
		str = data.(string)
	case map[string]any:
		str = MapToQuery(value)
	default:

	}
	return str
}

func MapToQuery(value map[string]any) string {
	mm := make([]string, 0)
	for k, v := range value {
		mm = append(mm, k+"="+fmt.Sprintf("%v", v))
	}
	return strings.Join(mm, "&")
}

func QueryToMap(str string) map[string]string {
	res := make(map[string]string)
	arr := strings.Split(str, "&")
	for _, v := range arr {
		vv := strings.Split(v, "=")
		res[vv[0]] = vv[1]
	}
	return res
}

func Session() *SessionStruct {
	return &SessionStruct{}
}

type SessionStruct struct {
	Cookies []*http.Cookie
}

func (ss *SessionStruct) Get(url string, args ...Args) Response {
	return ss.Request("GET", url, args...)
}

func (ss *SessionStruct) Post(url string, args ...Args) Response {
	return ss.Request("POST", url, args...)
}

func (ss *SessionStruct) Patch(url string, args ...Args) Response {
	return ss.Request("PATCH", url, args...)
}

func (ss *SessionStruct) Request(method string, urlStr string, args ...Args) Response {

	if len(args) == 0 {
		args = append(args, Args{})
	}
	ss.UpdateCookies(args[0].Cookies)
	args[0].Cookies = ss.Cookies

	//fmt.Println(args[0].Headers)
	res := Request(method, urlStr, args...)
	ss.UpdateCookies(res.Cookies)

	//fmt.Println(ss.Cookies)
	return res
}

func (ss *SessionStruct) UpdateCookies(cookies []*http.Cookie) {
	for i := 0; i < len(cookies); i++ {
		index := -1
		for j := 0; j < len(ss.Cookies); j++ {
			if cookies[i].Name == ss.Cookies[j].Name {
				index = j
				break
			}
		}

		if index == -1 {
			ss.Cookies = append(ss.Cookies, cookies[i])
		} else {
			ss.Cookies[index] = cookies[i]
		}
	}
}
