"""Local UI preview when Go is unavailable; never a production backend."""
from http.server import SimpleHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
import json,re
ROOT=Path(__file__).parent
fields=['id','player','account','community','reason','status','date','evidence']
records=[dict(zip(fields,re.findall(r'"([^"\n]*)"',line))) for line in (ROOT/'main.go').read_text(encoding='utf-8').splitlines() if line.strip().startswith('{"DEMO-')]
class Handler(SimpleHTTPRequestHandler):
 def __init__(self,*a,**kw): super().__init__(*a,directory=str(ROOT/'web'),**kw)
 def do_GET(self):
  if self.path.startswith('/api/v1/records'):
   body=json.dumps({'items':records,'total':len(records),'demo':True},ensure_ascii=False).encode()
   self.send_response(200);self.send_header('Content-Type','application/json');self.end_headers();self.wfile.write(body)
  else:super().do_GET()
if __name__=='__main__':ThreadingHTTPServer(('127.0.0.1',8088),Handler).serve_forever()
