package main

import (
	"fmt"
	"log"
	"net/http"
	"project/routes"
)

func main() {
	// 기본 라우팅 설정
	http.HandleFunc("/", routes.HomeRoute)
	http.HandleFunc("/task", routes.TaskRoute)

	// 서버 시작
	fmt.Println("Starting server on :8080...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
