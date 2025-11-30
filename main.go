package main

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Project define a single project with a URL and the language it was written in.
type Project struct {
	Added    time.Time `json:"added" readOnly:"true"`
	Language string    `json:"language" enum:"go,rust,python,typescript"`
	URL      string    `json:"url" format:"uri"`
}

// ProjectRecor is a MogoDB record for a project.
type ProjectRecord struct {
	Name    string  `json:"name"`
	Project Project `json:"project"`
}

// Response is a generic response type for the API with just a simple body.
type Response[T any] struct {
	Body T
}

// NewResponse returns the response type with the right body.
func NewResponse[T any](body T) *Response[T] {
	return &Response[T]{Body: body}
}

func main() {

	// Create a new Go HTTP servermux and Huma APi instance with default settings
	router := http.NewServeMux()
	api := humago.New(router, huma.DefaultConfig("My API", "1.0.0"))

	// Connect to MongoDB
	mongoClient, err := mongo.Connect(options.Client().
		ApplyURI(os.Getenv("MONGO_URI")).
		SetBSONOptions(&options.BSONOptions{
			UseJSONStructTags: true,
			DefaultDocumentM:  true,
		}))
	if err != nil {
		panic(err)
	}

	collection := mongoClient.Database("demo").Collection("projects")

	huma.Get(api, "/projects", func(ctx context.Context, input *struct {
		Language string `query:"language" enum:"go,rust,python,typescript" doc:"Filter by language"`
	}) (*Response[[]ProjectRecord], error) {
		var projects []ProjectRecord
		filter := bson.M{}
		if input.Language != "" {
			// Optional filtering by programming language.
			filter["project.language"] = input.Language
		}
		cursor, err := collection.Find(ctx, filter)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to fetch projects")
		}

		defer cursor.Close(ctx)
		if err := cursor.All(ctx, &projects); err != nil {
			return nil, huma.Error500InternalServerError("failed to decode projects")
		}

		return NewResponse(projects), nil
	})

	huma.Put(api, "/projects/{name}", func(ctx context.Context, input *struct {
		Name string `path:"name"`
		Body Project
	}) (*Response[Project], error) {
		input.Body.Added = time.Now()
		record := ProjectRecord{
			Name:    input.Name,
			Project: input.Body,
		}

		_, err := collection.InsertOne(ctx, record)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to insert project")
		}

		return NewResponse(record.Project), nil
	})

	// Start the server
	port := os.Getenv("PORT")
	if port == "" {
		port = "9050"
	}

	if err := http.ListenAndServe(":"+port, router); err != nil {
		panic(err)
	}

}
