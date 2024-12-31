package main

import (
	"context"
	"encoding/json"
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
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const uploadFileDir = "../file_path/upload_file_dir"
const deletedFileDir = "../file_path/deleted_file_dir"

// 필요한 필드 구조체 정의
type Document struct {
	OriginalFilename string `bson:"originalFilename"`
	StoredFileName   string `bson:"storedFileName"`
}

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
	err := client.Ping(context.Background(), nil)
	if err != nil {
		log.Fatal(err)
	}

	collection := client.Database("file").Collection("metadata")

	// 모든 문서 조회
	cursor, err := collection.Find(context.Background(), bson.D{})
	if err != nil {
		http.Error(w, "Unable to retrieve documents", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(context.Background())

	// 문서를 슬라이스에 저장
	var documents []bson.M
	for cursor.Next(context.Background()) {
		var document bson.M
		if err := cursor.Decode(&document); err != nil {
			http.Error(w, "Unable to decode document", http.StatusInternalServerError)
			return
		}
		documents = append(documents, document)
	}

	// cursor.Err() 확인
	if err := cursor.Err(); err != nil {
		http.Error(w, "Error reading from cursor", http.StatusInternalServerError)
		return
	}

	// JSON 형식으로 응답
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(documents)
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
	// err := r.ParseMultipartForm(10 << 20) // 최대 10MB까지 파일 업로드 처리
	// if err != nil {
	// 	http.Error(w, "Unable to parse multipart form", http.StatusBadRequest)
	// 	return
	// }

	// 요청 본문에서 form 데이터를 파싱합니다.
	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Failed to parse form data", http.StatusBadRequest)
		return
	}

	deleteFileId := r.FormValue("deleteFileId")
	if deleteFileId == "" {
		fmt.Println("Deleted File ID is Null")
		return
	}
	fmt.Print("Deleted File ID : ")
	fmt.Println(deleteFileId)

	err = client.Ping(context.Background(), nil)
	if err != nil {
		log.Fatal(err)
	}

	collection := client.Database("file").Collection("metadata")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// _id로 조회
	objectID, err := primitive.ObjectIDFromHex(deleteFileId)
	if err != nil {
		log.Fatalf("Invalid ObjectID: %v", err)
	}

	projection := bson.M{
		"originalFilename": 1,
		"storedFileName":   1,
	}

	// `_id`로 문서 조회 (프로젝션 사용)
	var result Document
	err = collection.FindOne(ctx, bson.M{"_id": objectID}, options.FindOne().SetProjection(projection)).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			fmt.Println("No document found with the specified _id")
		} else {
			log.Fatalf("Failed to find document: %v", err)
		}
		return
	}

	// 결과를 다른 변수에 저장하여 사용
	originalFilename := result.OriginalFilename
	storedFileName := result.StoredFileName

	// 업데이트 쿼리 실행
	currentTime := time.Now()
	formattedTime := currentTime.Format("20060102150405")
	filter := bson.M{"_id": objectID}
	update := bson.M{
		"$set": bson.M{
			"deletedTf": "true",
			"deletedAt": currentTime,
		},
	}
	result_update, err := collection.UpdateOne(ctx, filter, update)
	if err != nil {
		log.Fatal(err)
	}

	oldPath := uploadFileDir + "/" + storedFileName                          // 이동할 파일의 경로
	newPath := deletedFileDir + "/" + formattedTime + "_" + originalFilename // 이동할 파일의 대상 경로
	err = os.Rename(oldPath, newPath)
	if err != nil {
		fmt.Printf("Failed to move file: %v\n", err)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Printf("Successfully deleted %d document(s).\n", result_update)
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
		{"deletedAt", ""},
		{"deletedTf", false},
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
	http.HandleFunc("/download", downloadFile)
	http.HandleFunc("/delete", deleteFile)

	// 서버 시작
	fmt.Println("Server is listening on http://localhost:8080")
	err = http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("Error starting server:", err)
	}

}
