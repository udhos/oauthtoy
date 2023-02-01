/*
This is the main package for oauthtoy-server.
*/
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/golang-jwt/jwt"

	"github.com/udhos/oauthtoy/env"
)

const version = "0.0.0"

func getVersion(me string) string {
	return fmt.Sprintf("%s version=%s runtime=%s GOOS=%s GOARCH=%s GOMAXPROCS=%d",
		me, version, runtime.Version(), runtime.GOOS, runtime.GOARCH, runtime.GOMAXPROCS(0))
}

func main() {

	var showVersion bool
	flag.BoolVar(&showVersion, "version", showVersion, "show version")
	flag.Parse()

	me := filepath.Base(os.Args[0])

	{
		v := getVersion(me)
		if showVersion {
			fmt.Print(v)
			fmt.Println()
			return
		}
		log.Print(v)
	}

	addr := env.String("ADDR", ":8080")
	pathToken := env.String("ROUTE", "/oauth/token")

	mux := http.NewServeMux()
	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	const root = "/"

	register(mux, addr, root, handlerRoot)
	register(mux, addr, pathToken, handlerToken)
	register(mux, addr, "/echo", handlerEcho)

	go listenAndServe(server, addr)

	<-chan struct{}(nil)
}

func register(mux *http.ServeMux, addr, path string, handler http.HandlerFunc) {
	mux.HandleFunc(path, handler)
	log.Printf("registered on port %s path %s", addr, path)
}

func listenAndServe(s *http.Server, addr string) {
	log.Printf("listening on port %s", addr)
	err := s.ListenAndServe()
	log.Printf("listening on port %s: %v", addr, err)
}

// httpJSON replies to the request with the specified error message and HTTP code.
// It does not otherwise end the request; the caller should ensure no further
// writes are done to w.
// The message should be JSON.
func httpJSON(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(code)
	fmt.Fprintln(w, message)
}

func response(w http.ResponseWriter, r *http.Request, status int, message string) {
	hostname, errHost := os.Hostname()
	if errHost != nil {
		log.Printf("hostname error: %v", errHost)
	}
	reply := fmt.Sprintf(`{"message":"%s","status":"%d","path":"%s","method":"%s","host":"%s","serverHostname":"%s"}`,
		message, status, r.RequestURI, r.Method, r.Host, hostname)
	httpJSON(w, reply, status)
}

func handlerRoot(w http.ResponseWriter, r *http.Request) {
	log.Printf("%s %s %s - 404 not found", r.RemoteAddr, r.Method, r.RequestURI)
	response(w, r, http.StatusNotFound, "not found")
}

var sampleSecretKey = []byte("SecretYouShouldHide")

func handlerToken(w http.ResponseWriter, r *http.Request) {

	params := getParameters(r, "grant_type", "client_id", "client_secret")

	grantType := params["grant_type"]
	clientID := params["client_id"]
	clientSecret := params["client_secret"]

	log.Printf("method=%s grant_type=%s client_id=%s client_secret=%s",
		r.Method, grantType, clientID, clientSecret)

	if grantType != "client_credentials" {
		log.Printf("%s %s %s - wrong grant type - 401 unauthorized", r.RemoteAddr, r.Method, r.RequestURI)
		response(w, r, http.StatusUnauthorized, "unauthorized")
		return
	}

	if clientID != "admin" || clientSecret != "admin" {
		log.Printf("%s %s %s - bad credentials - 401 unauthorized", r.RemoteAddr, r.Method, r.RequestURI)
		response(w, r, http.StatusUnauthorized, "unauthorized")
		return
	}

	type format struct {
		AccessToken  string `json:"access_token"`
		TokenType    string `json:"token_type"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}

	/*
		const expire = 10

		accessToken := jwt.New(jwt.SigningMethodHS256)
		claims := accessToken.Claims.(jwt.MapClaims)
		claims["exp"] = time.Now().Add(expire * time.Second)
		//claims["authorized"] = true
		//claims["user"] = "username"
		accessTokenStr, errSign := accessToken.SignedString(sampleSecretKey)
		if errSign != nil {
			log.Printf("%s %s %s - sign access token - 500 server error: %v",
				r.RemoteAddr, r.Method, r.RequestURI, errSign)
			response(w, r, http.StatusInternalServerError, "server error")
			return
		}
	*/

	const expire = 30

	accessToken, errAccess := newToken(clientID, expire)
	if errAccess != nil {
		log.Printf("%s %s %s - access token - 500 server error: %v",
			r.RemoteAddr, r.Method, r.RequestURI, errAccess)
		response(w, r, http.StatusInternalServerError, "server error")
		return
	}

	refreshToken, errRefresh := newToken(clientID, 0)
	if errRefresh != nil {
		log.Printf("%s %s %s - refresh token - 500 server error: %v",
			r.RemoteAddr, r.Method, r.RequestURI, errRefresh)
		response(w, r, http.StatusInternalServerError, "server error")
		return
	}

	reply := format{
		AccessToken:  accessToken,
		TokenType:    "Bearer",
		RefreshToken: refreshToken,
		ExpiresIn:    expire,
	}

	buf, errJSON := json.Marshal(&reply)
	if errJSON != nil {
		log.Printf("%s %s %s - json error - 500 server error", r.RemoteAddr, r.Method, r.RequestURI)
		response(w, r, http.StatusInternalServerError, "server error")
		return
	}

	log.Printf("%s %s %s - 200 ok", r.RemoteAddr, r.Method, r.RequestURI)

	httpJSON(w, string(buf), http.StatusOK)
}

func newToken(clientID string, exp int) (string, error) {
	accessToken := jwt.New(jwt.SigningMethodHS256)
	claims := accessToken.Claims.(jwt.MapClaims)
	now := time.Now()
	claims["iat"] = now.Unix()
	if exp > 0 {
		claims["exp"] = now.Add(time.Duration(exp) * time.Second).Unix()
	}
	claims["client_id"] = clientID

	str, errSign := accessToken.SignedString(sampleSecretKey)
	if errSign != nil {
		return "", errSign
	}
	return str, nil
}

func getParameters(r *http.Request, keys ...string) map[string]string {

	var formParsed bool

	table := map[string]string{}

	for _, k := range keys {
		if value, found := r.URL.Query()[k]; found {
			v := value[0]
			table[k] = v
			continue
		}

		if !formParsed {
			log.Printf("parsing form")
			if err := r.ParseForm(); err != nil {
				log.Printf("getParameters: err: %v", err)
			}
			formParsed = true
		}

		table[k] = r.FormValue(k)
	}

	return table
}

func handlerEcho(w http.ResponseWriter, r *http.Request) {

	auth := r.Header.Get("Authorization")
	_, accessToken, _ := strings.Cut(auth, " ")

	log.Printf("%s %s %s - access token: %v", r.RemoteAddr, r.Method, r.RequestURI, accessToken)

	token, errParse := jwt.Parse(accessToken, func(token *jwt.Token) (interface{}, error) {
		return sampleSecretKey, nil
	})
	if errParse != nil {
		log.Printf("%s %s %s - parse access token: %v", r.RemoteAddr, r.Method, r.RequestURI, errParse)
	} else {
		log.Printf("%s %s %s - access token: valid:%t", r.RemoteAddr, r.Method, r.RequestURI, token.Valid)
	}

	if errParse != nil || !token.Valid {
		log.Printf("%s %s %s - bad access token - 401 unauthorized", r.RemoteAddr, r.Method, r.RequestURI)
		response(w, r, http.StatusUnauthorized, "unauthorized")
		return
	}

	buf, errJSON := json.Marshal(r.Header)
	if errJSON != nil {
		log.Printf("%s %s %s - json error - 500 server error", r.RemoteAddr, r.Method, r.RequestURI)
		response(w, r, http.StatusInternalServerError, "server error")
		return
	}

	log.Printf("%s %s %s - 200 ok", r.RemoteAddr, r.Method, r.RequestURI)

	httpJSON(w, string(buf), http.StatusOK)
}
