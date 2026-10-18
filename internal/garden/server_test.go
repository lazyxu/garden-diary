package garden

import (
    "archive/zip"
    "bytes"
    "encoding/json"
    "image"
    "image/color"
    "image/png"
    "mime/multipart"
    "net/http"
    "net/http/httptest"
    "net/url"
    "strings"
    "strconv"
    "testing"
)
type testClient struct {
    app *App
    cookie *http.Cookie
    csrf string
}
func makeTestClient(t *testing.T)*testClient {
    t.Helper()
    app,err:=New(Config{DataDir:t.TempDir(),SetupToken:"test-setup-secret"})
    if err!=nil{t.Fatal(err)}
    t.Cleanup(func(){app.Close()})
    return &testClient{app:app}
}
func(c *testClient) request(t *testing.T,method,path string,body []byte,contentType string,authenticated bool)*httptest.ResponseRecorder {
    t.Helper()
    req:=httptest.NewRequest(method,"http://garden.local"+path,bytes.NewReader(body))
    if contentType!=""{req.Header.Set("Content-Type",contentType)}
    if authenticated {
        if c.cookie!=nil{req.AddCookie(c.cookie)}
        if method!="GET"{req.Header.Set("X-CSRF-Token",c.csrf)}
    }
    resp:=httptest.NewRecorder()
    c.app.ServeHTTP(resp,req)
    return resp
}
func(c *testClient) postJSON(t *testing.T,path string,value any)*httptest.ResponseRecorder {
    t.Helper()
    encoded,err:=json.Marshal(value);if err!=nil{t.Fatal(err)}
    return c.request(t,"POST",path,encoded,"application/json",true)
}
func requireStatus(t *testing.T,r *httptest.ResponseRecorder,want int) {
    t.Helper()
    if r.Code!=want{t.Fatalf("HTTP %d want %d: %s",r.Code,want,r.Body.String())}
}
func(c *testClient) authenticate(t *testing.T,r *httptest.ResponseRecorder) {
    t.Helper()
    requireStatus(t,r,200)
    var session struct{CSRF string `json:"csrf"`}
    if err:=json.Unmarshal(r.Body.Bytes(),&session);err!=nil{t.Fatal(err)}
    if session.CSRF==""{t.Fatal("missing CSRF token")}
    c.csrf=session.CSRF
    cookies:=r.Result().Cookies()
    if len(cookies)==0{t.Fatal("missing session cookie")}
    c.cookie=cookies[0]
    if !c.cookie.HttpOnly {t.Fatal("cookie must be HttpOnly")}
}
func setupTestAdmin(t *testing.T,c *testClient) {
    t.Helper()
    r:=c.postJSON(t,"/api/v1/setup",map[string]any{"setup_token":"test-setup-secret","username":"gardener","display_name":"园丁","password":"secret-password-123"})
    c.authenticate(t,r)
}
func TestBootstrapSessionAndCSRF(t *testing.T){
    c:=makeTestClient(t)
    r:=c.request(t,"GET","/api/v1/bootstrap",nil,"",false)
    requireStatus(t,r,200)
    if !strings.Contains(r.Body.String(),"true"){t.Fatal("should need initial setup")}
    requireStatus(t,c.postJSON(t,"/api/v1/setup",map[string]any{"setup_token":"bad","username":"gardener","display_name":"园丁","password":"secret-password-123"}),403)
    setupTestAdmin(t,c)
    requireStatus(t,c.postJSON(t,"/api/v1/setup",map[string]any{"setup_token":"test-setup-secret","username":"other","display_name":"其他","password":"secret-password-123"}),409)
    r=c.request(t,"GET","/api/v1/me",nil,"",true);requireStatus(t,r,200)
    if !strings.Contains(r.Body.String(),"gardener"){t.Fatal("missing identity")}
    payload:=[]byte(`{"name":"月季"}`)
    requireStatus(t,c.request(t,"POST","/api/v1/species",payload,"application/json",false),401)
    req:=httptest.NewRequest("POST","http://garden.local/api/v1/species",bytes.NewReader(payload))
    req.Header.Set("Content-Type","application/json");req.AddCookie(c.cookie)
    bad:=httptest.NewRecorder();c.app.ServeHTTP(bad,req);requireStatus(t,bad,403)
    req=httptest.NewRequest("POST","http://garden.local/api/v1/species",bytes.NewReader(payload))
    req.Header.Set("Content-Type","application/json")
    req.Header.Set("Origin","http://hostile.example")
    req.Header.Set("X-CSRF-Token",c.csrf);req.AddCookie(c.cookie)
    bad=httptest.NewRecorder();c.app.ServeHTTP(bad,req);requireStatus(t,bad,403)
}
func TestFamilyPlantsPhotoExport(t *testing.T){
    c:=makeTestClient(t);setupTestAdmin(t,c)
    res:=c.postJSON(t,"/api/v1/species",map[string]any{"name":"月季"})
    requireStatus(t,res,201)
    res=c.postJSON(t,"/api/v1/plants",map[string]any{"name":"阳台月季","species_id":1,"planted_on":"2026-05-01","location":"阳台","notes":"第一株","status":"active"})
    requireStatus(t,res,201)
    pngBytes:=new(bytes.Buffer)
    img:=image.NewRGBA(image.Rect(0,0,16,16))
    img.Set(5,5,color.RGBA{R:255,A:255})
    if err:=png.Encode(pngBytes,img);err!=nil{t.Fatal(err)}
    body:=new(bytes.Buffer)
    writer:=multipart.NewWriter(body)
    for key,value:=range map[string]string{"plant_id":"1","observed_at":"2026-10-18T10:30:00+09:00","note":"开花了"}{
        if err:=writer.WriteField(key,value);err!=nil{t.Fatal(err)}
    }
    part,err:=writer.CreateFormFile("photos","flower.png");if err!=nil{t.Fatal(err)}
    if _,err:=part.Write(pngBytes.Bytes());err!=nil{t.Fatal(err)}
    if err:=writer.Close();err!=nil{t.Fatal(err)}
    res=c.request(t,"POST","/api/v1/entries",body.Bytes(),writer.FormDataContentType(),true)
    requireStatus(t,res,201)
    res=c.request(t,"GET","/api/v1/entries?plant_id=1",nil,"",true)
    requireStatus(t,res,200)
    var page struct{Items []Entry `json:"items"`}
    if err:=json.Unmarshal(res.Body.Bytes(),&page);err!=nil{t.Fatal(err)}
    if len(page.Items)!=1||len(page.Items[0].Photos)!=1{t.Fatalf("unexpected entries: %s",res.Body.String())}
    photoID:=page.Items[0].Photos[0].ID
    res=c.request(t,"GET","/api/v1/photos/"+fmtID(photoID)+"/thumb",nil,"",true)
    requireStatus(t,res,200)
    if !strings.HasPrefix(res.Header().Get("Content-Type"),"image/jpeg"){t.Fatal("missing thumbnail")}
    res=c.request(t,"GET","/api/v1/photos/"+fmtID(photoID)+"/preview",nil,"",true)
    requireStatus(t,res,200)
    if !strings.HasPrefix(res.Header().Get("Content-Type"),"image/png"){t.Fatal("JPEG/PNG previews must use original type")}
    res=c.request(t,"GET","/api/v1/calendar/day?date=2026-10-18",nil,"",true)
    requireStatus(t,res,200)
    if !strings.Contains(res.Body.String(),"阳台月季"){t.Fatal("calendar day missing record")}
    res=c.request(t,"GET","/api/v1/export",nil,"",true)
    requireStatus(t,res,200)
    archive,err:=zip.NewReader(bytes.NewReader(res.Body.Bytes()),int64(res.Body.Len()))
    if err!=nil{t.Fatal(err)}
    if len(archive.File)<2{t.Fatal("export should contain manifest and photo")}
    res=c.postJSON(t,"/api/v1/users",map[string]any{"username":"family","display_name":"家人","password":"some-password-123","role":"member"})
    requireStatus(t,res,201)
    other:=&testClient{app:c.app}
    response:=other.postJSON(t,"/api/v1/login",map[string]any{"username":"family","password":"some-password-123"})
    other.authenticate(t,response)
    res=other.request(t,"GET","/api/v1/plants",nil,"",true)
    requireStatus(t,res,200)
    if !strings.Contains(res.Body.String(),"阳台月季"){t.Fatal("family member cannot view shared plant")}
    res=other.postJSON(t,"/api/v1/users",map[string]any{"username":"extra","display_name":"朋友","password":"some-password-123"})
    requireStatus(t,res,403)
    // URL encoded query cannot bypass the date constraint.
    res=c.request(t,"GET","/api/v1/calendar/day?date="+url.QueryEscape("../2026-10-18"),nil,"",true)
    requireStatus(t,res,400)
}
func fmtID(id int64)string{
    return strconv.FormatInt(id,10)
}
