http = require('http')
Buffer = require('buffer').Buffer;
n = 500;
b = new Buffer(n);
for (var i = 0; i < n; i++) b[i] = 100;
 
http.createServer(function (req, res) {
  res.writeHead(200);
  res.write(b);
  res.end();
}).listen(8080, function() {
  console.log("Listening");
});