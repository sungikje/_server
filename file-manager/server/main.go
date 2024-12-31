package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const uploadFileDir = "../file_path/upload_file_dir"
const deletedFileDir = "../file_path/deleted_file_dir"

var client *mongo.Client // 전역 변수로 MongoDB 클라이언트 선언

func getFileSize(file multipart.File) (int64, error) {
	// 더 큰 버퍼 크기 (예: 1MB)
	buf := make([]byte, 1024*1024) // 1MB 버퍼 크기
	var size int64

	for {
		n, err := file.Read(buf)
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, err
		}
		size += int64(n)
	}

	return size, nil
}

// 파일 업로드 처리
func uploadFile(w http.ResponseWriter, r *http.Request) {
	err := r.ParseMultipartForm(80 << 20) // 80MB 제한
	if err != nil {
		http.Error(w, "Unable to parse form", http.StatusBadRequest)
		return
	}

	// file, header, err := r.FormFile("file")
	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Unable to read file", http.StatusBadRequest)
		return
	}

	fileSize, err := getFileSize(file)
	if err != nil {
		log.Fatal(err)
	}

	defer file.Close()

	originalFileName := r.FormValue("file_name")

	currentTime := time.Now()
	formattedTime := currentTime.Format("20060102150405")
	storedFileName := formattedTime + "_" + originalFileName

	// 저장할 파일 경로 설정
	filePath := filepath.Join(uploadFileDir, storedFileName)
	outFile, err := os.Create(filePath)
	if err != nil {
		http.Error(w, "Unable to save file", http.StatusInternalServerError)
		return
	}
	defer outFile.Close()

	// 파일 포인터를 처음으로 이동
	_, err = file.Seek(0, io.SeekStart)
	if err != nil {
		http.Error(w, "Unable to seek file", http.StatusInternalServerError)
		return
	}

	_, err = io.Copy(outFile, file)
	if err != nil {
		http.Error(w, "Unable to write file", http.StatusInternalServerError)
		return
	}

	insertFileMetadata(originalFileName, storedFileName, filePath, fileSize, currentTime)

	// 응답
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "File uploaded successfully")
}

// 파일 검색 처리 (업로드된 파일 목록)
func searchFile(w http.ResponseWriter, r *http.Request) {
	// func searchFile() {
	fmt.Println("call search")
	// mongodb metadata 불러오기
}

// 파일 다운로드 처리
func downloadFile(w http.ResponseWriter, r *http.Request) {
	// URL에서 파일 ID를 가져오기 (예시: /download/1)
	segments := r.URL.Path[len("/download/"):]
	fileID, err := strconv.Atoi(segments)
	if err != nil {
		http.Error(w, "Invalid file ID", http.StatusBadRequest)
		return
	}

	// 다운로드할 파일 경로 설정
	filePath := filepath.Join(uploadFileDir, "uploaded_file"+strconv.Itoa(fileID))
	file, err := os.Open(filePath)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}
	defer file.Close()

	// 파일 내용 전송
	w.Header().Set("Content-Disposition", "attachment; filename="+filepath.Base(filePath))
	w.Header().Set("Content-Type", "application/octet-stream")
	_, err = io.Copy(w, file)
	if err != nil {
		http.Error(w, "Unable to send file", http.StatusInternalServerError)
		return
	}
}

// 파일 삭제 처리
func deleteFile(w http.ResponseWriter, r *http.Request) {
	// URL에서 파일 ID를 가져오기 (예시: /delete/1)
	segments := r.URL.Path[len("/delete/"):]
	fileID, err := strconv.Atoi(segments)
	if err != nil {
		http.Error(w, "Invalid file ID", http.StatusBadRequest)
		return
	}

	// 삭제할 파일 경로 설정
	filePath := filepath.Join(uploadFileDir, "uploaded_file"+strconv.Itoa(fileID))
	err = os.Remove(filePath)
	if err != nil {
		http.Error(w, "File not found or unable to delete", http.StatusNotFound)
		return
	}

	// mongodb metadata 제거

	// 응답
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "File deleted successfully")
}

func connectMongoDB() {
	clientOptions := options.Client().ApplyURI("mongodb://localhost:27017")
	// 여기서 client := 이용하는 경우 내부 스코프로 한정, = 사용해야되며 err도 미리 선언 필요
	var err error
	client, err = mongo.Connect(context.Background(), clientOptions)
	if err != nil {
		log.Fatal(err)
	}
	// defer client.Disconnect(context.Background())
	fmt.Println("Successfully connected to MongoDB!")
}

func insertFileMetadata(fileName string, storedFileName string, filePath string, fileSize int64, currentTime time.Time) {
	// MongoDB 연결 확인
	err := client.Ping(context.Background(), nil)
	if err != nil {
		log.Fatal(err)
	}

	// 'test' 데이터베이스와 'users' 컬렉션 참조
	collection := client.Database("file").Collection("metadata")
	// 결과 출력
	// fmt.Println("Formatted Date:", formattedDate)

	// 예시 데이터
	user := bson.D{
		{"originalFilename", fileName},
		{"storedFileName", storedFileName},
		{"fileSize", fileSize},
		{"path", filePath},
		{"uploadedAt", currentTime},
	}

	// 데이터 삽입
	insertResult, err := collection.InsertOne(context.Background(), user)
	if err != nil {
		log.Fatal(err)
	}

	// 삽입된 데이터의 ID 출력
	fmt.Printf("Inserted document with ID: %v\n", insertResult.InsertedID)
}

func main() {
	// 업로드 파일을 저장할 디렉토리 확인
	err := os.MkdirAll(uploadFileDir, os.ModePerm)
	if err != nil {
		log.Fatal("Unable to create file directory:", err)
	}

	connectMongoDB()

	// HTTP 핸들러 등록
	http.HandleFunc("/upload", uploadFile)
	http.HandleFunc("/search", searchFile)
	http.HandleFunc("/download/", downloadFile)
	http.HandleFunc("/delete/", deleteFile)

	// 서버 시작
	fmt.Println("Server is listening on http://localhost:8080")
	err = http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("Error starting server:", err)
	}

}
