package handlers

import (
	"fmt"
	"net/http"
	"time"
)

// HandleTask: /task 요청 처리
func HandleTask(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Handling /task request...\n")

	// 새 고루틴 생성
	go performTask()

	fmt.Fprintf(w, "Task is running in a separate thread.\n")
}

// performTask: 고루틴으로 실행될 함수
func performTask() {
	fmt.Println("Task started...")
	time.Sleep(5 * time.Second) // 작업 시뮬레이션
	fmt.Println("Task completed!")
}
