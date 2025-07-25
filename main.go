package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Todo struct {
	ID        primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	Completed bool               `json:"completed" bson:"completed"`
	Body      string             `json:"body" bson:"body"`
	CreatedAt time.Time          `json:"created_at" bson:"created_at"`
	UpdatedAt time.Time          `json:"updated_at" bson:"updated_at"`
}

type App struct {
	client     *mongo.Client
	collection *mongo.Collection
}

func main() {
	fmt.Println("Server is Running")

	if err := godotenv.Load(".env"); err != nil {
		log.Println("No .env file found - using system environment variables")
	}

	mongoURI := os.Getenv("MONGODB_URI")
	port := os.Getenv("PORT")

	if mongoURI == "" {
		log.Fatal("MONGODB_URI environment variable is required")
	}
	if port == "" {
		port = "5000"
	}

	// Initialize MongoDB connection
	app := &App{}
	if err := app.connectDB(); err != nil {
		log.Fatal("Failed to connect to MongoDB:", err)
	}
	defer app.client.Disconnect(context.Background())

	// Initialize Fiber app
	fiberApp := fiber.New()

	// Add CORS middleware
	fiberApp.Use(cors.New())

	// Setup routes
	fiberApp.Get("/api/todos", app.getTodos)
	fiberApp.Post("/api/todos", app.createTodo)
	fiberApp.Patch("/api/todos/:id", app.updateTodo)
	fiberApp.Delete("/api/todos/:id", app.deleteTodo)

	fmt.Printf("Server starting on port %s\n", port)
	log.Fatal(fiberApp.Listen(":" + port))
}

func (app *App) connectDB() error {
	mongoURI := os.Getenv("MONGODB_URI")
	if mongoURI == "" {
		mongoURI = "mongodb://admin:password123@localhost:27017/todoapp?authSource=admin"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		return err
	}

	// Ping the database
	if err := client.Ping(ctx, nil); err != nil {
		return err
	}

	app.client = client
	app.collection = client.Database("todoapp").Collection("todos")

	fmt.Println("Connected to MongoDB!")
	return nil
}

func (app *App) getTodos(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cursor, err := app.collection.Find(ctx, bson.M{})
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch todos"})
	}
	defer cursor.Close(ctx)

	var todos []Todo
	if err := cursor.All(ctx, &todos); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to decode todos"})
	}

	// Return empty array if no todos found
	if todos == nil {
		todos = []Todo{}
	}

	return c.Status(200).JSON(todos)
}

func (app *App) createTodo(c *fiber.Ctx) error {
	todo := &Todo{}

	if err := c.BodyParser(&todo); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body!"})
	}

	if todo.Body == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Todo body is required!"})
	}

	// Set timestamps
	now := time.Now()
	todo.CreatedAt = now
	todo.UpdatedAt = now
	todo.Completed = false // Default to false

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := app.collection.InsertOne(ctx, todo)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create todo"})
	}

	todo.ID = result.InsertedID.(primitive.ObjectID)

	return c.Status(201).JSON(todo)
}

func (app *App) updateTodo(c *fiber.Ctx) error {
	id := c.Params("id")

	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid todo ID"})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Update the todo to mark as completed and update timestamp
	update := bson.M{
		"$set": bson.M{
			"completed":  true,
			"updated_at": time.Now(),
		},
	}

	result, err := app.collection.UpdateOne(ctx, bson.M{"_id": objectID}, update)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to update todo"})
	}

	if result.MatchedCount == 0 {
		return c.Status(404).JSON(fiber.Map{"error": "Todo not found"})
	}

	// Fetch and return updated todo
	var todo Todo
	err = app.collection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&todo)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch updated todo"})
	}

	return c.Status(200).JSON(todo)
}

func (app *App) deleteTodo(c *fiber.Ctx) error {
	id := c.Params("id")

	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid todo ID"})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := app.collection.DeleteOne(ctx, bson.M{"_id": objectID})
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to delete todo"})
	}

	if result.DeletedCount == 0 {
		return c.Status(404).JSON(fiber.Map{"error": "Todo not found"})
	}

	return c.Status(200).JSON(fiber.Map{"message": "Delete successful"})
}
