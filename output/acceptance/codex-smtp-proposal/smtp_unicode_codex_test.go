package email
import("io";"mime";"mime/quotedprintable";"strings";"testing")
func TestCodexSMTPUnicodeRoundTrip(t *testing.T){
 for _,value:=range []string{"Bonjour André — véhicule prêt ✓","line one\r\nline two café","trailing space \r\n",strings.Repeat("é",200)}{
 encoded:=encodeBody(value)
 decoded,err:=io.ReadAll(quotedprintable.NewReader(strings.NewReader(encoded)));if err!=nil{t.Fatal(err)}
 // MIME quoted printable normalizes physical line endings to CRLF.
 expected:=strings.ReplaceAll(strings.ReplaceAll(value,"\r\n","\n"),"\n","\r\n")
 if string(decoded)!=expected{t.Fatalf("body roundtrip changed content: %q != %q",decoded,expected)}
 header:=mime.QEncoding.Encode("UTF-8",value)
 plain,err:=new(mime.WordDecoder).DecodeHeader(header);if err!=nil||plain!=value{t.Fatalf("subject roundtrip changed content: %q, %v",plain,err)}
 for _,line:=range strings.Split(encoded,"\r\n"){if len(line)>76{t.Fatalf("MIME physical line too long: %d",len(line))}}
 }
}
