
package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/joho/godotenv"
	"github.com/thedevsaddam/renderer"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var rnd *renderer.Render
var client *mongo.Client
var db *mongo.Database

const (
	hostname       string = "localhost"
	dbName         string = "demo_todo"
	collectionName string = "todo"
	port           string = ":9010"
)

type (
	todoModel struct {
		ID        primitive.ObjectID `bson:"_id,omitempty"`
		Title     string             `bson:"title"`
		Completed bool               `bson:"completed"`
		CreateAt  time.Time          `bson:"createAt"`
	}
	todo struct {
		ID        string    `json:"id"`
		Title     string    `json:"title"`
		Completed bool      `json:"completed"`
		CreatedAt time.Time `json:"create_at"`
	}
)

func init() {
	// Load environment variables from .env file
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	mongoURI := os.Getenv("MONGO_URI")
	if mongoURI == "" {
		log.Fatal("MONGO_URI environment variable is not set")
	}

	rnd = renderer.New()

	clientOptions := options.Client().ApplyURI(mongoURI)
	client, err := mongo.Connect(context.Background(), clientOptions)
	if err != nil {
		log.Fatal(err)
	}

	db = client.Database(dbName)
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	err := rnd.Template(w, http.StatusOK, []string{"./static/home.tpl"}, nil)
	checkErr(err)
}

func fetchTodos(w http.ResponseWriter, r *http.Request) {
	collection := db.Collection(collectionName)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cur, err := collection.Find(ctx, bson.M{})
	if err != nil {
		rnd.JSON(w, http.StatusInternalServerError, renderer.M{
			"message": "Failed to fetch todo",
			"error":   err.Error(),
		})
		return
	}
	defer cur.Close(ctx)

	var todos []todoModel
	if err := cur.All(ctx, &todos); err != nil {
		rnd.JSON(w, http.StatusInternalServerError, renderer.M{
			"message": "Failed to decode todos",
			"error":   err.Error(),
		})
		return
	}

	var todoList []todo
	for _, t := range todos {
		todoList = append(todoList, todo{
			ID:        t.ID.Hex(),
			Title:     t.Title,
			Completed: t.Completed,
			CreatedAt: t.CreateAt,
		})
	}

	rnd.JSON(w, http.StatusOK, renderer.M{"data": todoList})
}
func createTodos(w http.ResponseWriter, r *http.Request) {
	var t todo
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		rnd.JSON(w, http.StatusBadRequest, renderer.M{"message": "Invalid request payload"})
		return
	}

	if t.Title == "" {
		rnd.JSON(w, http.StatusBadRequest, renderer.M{"message": "Title field is required"})
		return
	}

	tm := todoModel{
		ID:        primitive.NewObjectID(),
		Title:     t.Title,
		Completed: false,
		CreateAt:  time.Now(),
	}

	collection := db.Collection(collectionName)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := collection.InsertOne(ctx, tm)
	if err != nil {
		rnd.JSON(w, http.StatusInternalServerError, renderer.M{"message": "Failed to save todo", "error": err.Error()})
		return
	}

	rnd.JSON(w, http.StatusOK, renderer.M{"message": "Todo successfully saved", "Todo ID": tm.ID.Hex()})
}

func deleteTodo(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if !primitive.IsValidObjectID(id) {
		rnd.JSON(w, http.StatusBadRequest, renderer.M{"message": "Invalid ID"})
		return
	}

	objectID, _ := primitive.ObjectIDFromHex(id)
	collection := db.Collection(collectionName)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res, err := collection.DeleteOne(ctx, bson.M{"_id": objectID})
	if err != nil {
		rnd.JSON(w, http.StatusInternalServerError, renderer.M{"message": "Failed to delete TODO", "error": err.Error()})
		return
	}

	if res.DeletedCount == 0 {
		rnd.JSON(w, http.StatusNotFound, renderer.M{"message": "Todo not found"})
		return
	}

	rnd.JSON(w, http.StatusOK, renderer.M{"message": "Successfully deleted TODO"})
}

func updateTodo(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if !primitive.IsValidObjectID(id) {
		rnd.JSON(w, http.StatusBadRequest, renderer.M{"message": "Invalid ID"})
		return
	}

	var t todo
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		rnd.JSON(w, http.StatusBadRequest, renderer.M{"message": "Invalid request payload"})
		return
	}

	if t.Title == "" {
		rnd.JSON(w, http.StatusBadRequest, renderer.M{"message": "Title field is required"})
		return
	}

	objectID, _ := primitive.ObjectIDFromHex(id)
	collection := db.Collection(collectionName)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	update := bson.M{"$set": bson.M{"title": t.Title, "completed": t.Completed}}
	_, err := collection.UpdateOne(ctx, bson.M{"_id": objectID}, update)
	if err != nil {
		rnd.JSON(w, http.StatusInternalServerError, renderer.M{"message": "Failed to update todo", "error": err.Error()})
		return
	}

	rnd.JSON(w, http.StatusOK, renderer.M{"message": "Successfully updated TODO"})
}

func main() {
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt)

	r := chi.NewRouter()
	r.Use(middleware.Logger)

	r.Get("/", homeHandler)
	r.Mount("/todo", todoHandlers())

	srv := &http.Server{
		Addr:         port,
		Handler:      r,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Println("Listening on Port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen %s \n", err)
		}
	}()

	<-stopChan
	log.Println("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server shutdown failed:%+v", err)
	}
	log.Println("Server Gracefully stopped!!")
}

func todoHandlers() http.Handler {
	rg := chi.NewRouter()

	rg.Group(func(r chi.Router) {
		r.Get("/", fetchTodos)
		r.Post("/", createTodos)
		r.Put("/{id}", updateTodo)
		r.Delete("/{id}", deleteTodo)
	})
	return rg
}

func checkErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
