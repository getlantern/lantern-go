package persona

import (
	"encoding/json"
	"fmt"
	"github.com/toqueteos/webbrowser"
	"io/ioutil"
	"lantern/config"
	"log"
	"net/http"
	"net/url"
)

var assertionResult = make(chan string)

func init() {
	http.HandleFunc("/auth", indexHandler)
	http.HandleFunc("/auth/login", loginHandler)
	go http.ListenAndServe(config.UIAddress(), nil)
}

type PersonaResponse struct {
	Status   string `json: "status"`
	Email    string `json: "email"`
	Audience string `json: "audience"`
	Expires  int64  `json: "expires"`
	Issuer   string `json: "issuer"`
}

func GetIdentityAssertion() chan string {
	url := "http://" + config.UIAddress() + "/auth"
	log.Printf("Opening browser to: %s", url)
	webbrowser.Open(url)
	return assertionResult
}

var template = `
<html>
  <head>
    <title>Mozilla Persona Test</title>
	<meta http-equiv="X-UA-Compatible" content="IE=Edge">
  </head>
  <body>
  	<div id="loggedOut">
	    <h1 id="title">Please Log In using Mozilla Persona.</h1>
	    <a href="#" id="login">login</a>
	    <a href="#" id="logout">logout</a>
	</div>
	<div id="loggedIn" style="display: none;">
		<h1>Thank you for logging in!</h1>
	</div>
  
    <script src="https://login.persona.org/include.js"></script>
    <script>
	    var signinLink = document.getElementById('login');
		if (signinLink) {
		  signinLink.onclick = function() { navigator.id.request(); };
		}
		
		var signoutLink = document.getElementById('logout');
		if (signoutLink) {
		  signoutLink.onclick = function() { navigator.id.logout(); };
		}
		
		var loggedOutDiv = document.getElementById('loggedOut');
		var loggedInDiv = document.getElementById('loggedIn');
		
		function simpleXhrSentinel(xhr) {
		    return function() {
		        if (xhr.readyState == 4) {
		            if (xhr.status == 200){
		                loggedOutDiv.style.display = "none";
		                loggedInDiv.style.display = "inherit";
		              }
		            else {
		                navigator.id.logout();
		                alert("XMLHttpRequest error: " + xhr.status); 
		              } 
		            } 
		          } 
		        }
		
		function verifyAssertion(assertion) {
		    // Your backend must return HTTP status code 200 to indicate successful
		    // verification of user's email address and it must arrange for the binding
		    // of currentUser to said address when the page is reloaded
		    var xhr = new XMLHttpRequest();
		    xhr.open("POST", "/auth/login", true);
		    // see http://www.openjs.com/articles/ajax_xmlhttp_using_post.php
		    var param = "assertion="+assertion;
		    xhr.setRequestHeader("Content-type", "application/x-www-form-urlencoded");
		    xhr.send(param); // for verification by your backend
		
		    xhr.onreadystatechange = simpleXhrSentinel(xhr); }
		
		function signoutUser() {
		    // Your backend must return HTTP status code 200 to indicate successful
		    // sign out (usually the resetting of one or more session variables) and
		    // it must arrange for the binding of currentUser to 'null' when the page
		    // is reloaded
		    var xhr = new XMLHttpRequest();
		    xhr.open("GET", "/auth/logout", true);
		    xhr.send(null);
		    xhr.onreadystatechange = simpleXhrSentinel(xhr); }
		
		// Go!
		navigator.id.watch( {
		    loggedInUser: null,
		         onlogin: verifyAssertion,
		        onlogout: signoutUser } );

    </script>
  </body>
</html>
`

func indexHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, template)
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {
	config.SetEmail("")
	w.Write([]byte("OK"))
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Login handler called")
	if err := r.ParseForm(); err != nil {
		log.Println(err)
		w.WriteHeader(400)
		w.Write([]byte("Bad Request."))
	}

	assertion := r.FormValue("assertion")
	if assertion == "" {
		log.Println("Didn't get assertion")
		w.WriteHeader(400)
		w.Write([]byte("Bad Request."))
	}

	pr, err := ValidateAssertion(assertion)
	if err != nil {
		log.Println(err)
		w.WriteHeader(400)
		w.Write([]byte("Bad Request."))
	} else {
		if prJson, err := json.Marshal(pr); err != nil {
			log.Println(err)
			w.WriteHeader(400)
			w.Write([]byte("Bad Request."))
		} else {
			log.Println("Identity assertion successfully verified")
			config.SetEmail(pr.Email)
			log.Println("Email saved")
			w.Write(prJson)
			log.Println("Response written")
			assertionResult <- assertion
		}
	}
}

func ValidateAssertion(assertion string) (*PersonaResponse, error) {
	data := url.Values{"assertion": {assertion}, "audience": {config.UIAddress()}}

	resp, err := http.PostForm("https://verifier.login.persona.org/verify", data)
	if err != nil {
		return nil, err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	pr := &PersonaResponse{}
	err = json.Unmarshal(body, pr)
	if err != nil {
		return nil, err
	}

	return pr, nil
}
