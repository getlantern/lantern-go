https = require('https')
var fs = require('fs');

var options = {
  key: fs.readFileSync('testconfigs/sponsor/keys/own/privatekey.pem'),
  cert: fs.readFileSync('testconfigs/sponsor/keys/own/certificate.pem')
};

Buffer = require('buffer').Buffer;
n = 500;
b = new Buffer(n);
for (var i = 0; i < n; i++) b[i] = 100;
 
https.createServer(options, function (req, res) {
  res.writeHead(200);
  res.write(b);
  res.end();
}).listen(8080, function() {
  console.log("Listening");
});