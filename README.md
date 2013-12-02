This is a proof of concept of a Go-based version of Lantern that uses a
distributed trust network.

Use go version 1.2 from [here](http://golang.org/doc/install).

Prerequisites
=============

Run the following inside your `lantern-go` folder:

```
export GOPATH=`pwd`
go get code.google.com/p/go.net/websocket
go get github.com/toqueteos/webbrowser
```

Building
========

```
export GOPATH=`pwd`
go install lantern
```

Running the Example
===================

The example involves a master and a user node.  Node "1" is the master, node
"1.1" is the user node.

To start the master:

```
bin/lantern testconfigs/1
```

To start the child:

```
mkdir -p testconfigs/1.1/keys/trusted
cp testconfigs/1/keys/own/certificate.pem testconfigs/1.1/keys/trusted/parentcert.pem
bin/lantern testconfigs/1.1
```

When starting the child for the first time, you should see a browser window open
and prompt you to log in with Mozilla Persona.  Log in with whatever email address
you like.