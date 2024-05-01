package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/sashabaranov/go-openai"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	// Define command-line flags
	var (
		mongoURIFlag     = flag.String("mongoURI", "mongodb://127.0.0.1:27017", "MongoDB connection URI")
		dbNameFlag       = flag.String("dbName", "default", "MongoDB database name")
		aiProvider       = flag.String("aiProvider", "openai", "AI provider to use (ollama, openai)")
		ollamaModel      = flag.String("ollamaModel", "llama3", "Ollama model to use: llama3, mistral, etc")
		openaiApiKeyFlag = flag.String("openaiApiKey", "", "OpenAI API key")
		openaiMaxTokens  = flag.Int("openaiMaxTokens", 500, "OpenAI maximum tokens per query")
		millis           = flag.Int("millis", 0, "Process queries with execution time >= millis")
	)
	flag.Parse()

	// MongoDB connection URI
	uri := *mongoURIFlag

	// Set MongoDB client options
	mongoClientOptions := options.Client().ApplyURI(uri)

	// Connect to MongoDB
	mongoClient, err := mongo.Connect(context.Background(), mongoClientOptions)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err = mongoClient.Disconnect(context.Background()); err != nil {
			log.Fatal(err)
		}
	}()

	// Ping the MongoDB server to check if the connection was successful
	err = mongoClient.Ping(context.Background(), nil)
	if err != nil {
		log.Fatal(err)
	}

	// Access the "system.profile" collection
	collection := mongoClient.Database(*dbNameFlag).Collection("system.profile")

	// Create a context with a timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Distinct queryHash values from the system.profile collection
	distinctQueryHash, err := collection.Distinct(ctx, "queryHash", bson.D{
		{Key: "op", Value: "query"},
		{Key: "command", Value: bson.M{"$exists": true}},
		{Key: "command.getMore", Value: bson.M{"$exists": false}},
		{Key: "millis", Value: bson.M{"$gte": *millis}},
	})
	if err != nil {
		log.Fatal("Failed to fetch queryHash from system.profile collection: ", err)
	}

	if len(distinctQueryHash) == 0 {
		fmt.Printf("No suitable queryHash found in %s db", *dbNameFlag)
		fmt.Println()
		os.Exit(0)
	}

	// Iterate over queryHash values
	fmt.Println("Distinct queryHash values:", distinctQueryHash)
	fmt.Println()

	for _, hash := range distinctQueryHash {
		fmt.Println("Query Hash:", hash)

		// Find one document where queryHash equals a specific value
		filter := bson.M{
			"queryHash": hash,
			"millis":    bson.M{"$gte": *millis},
		}
		projection := bson.M{
			"op":      1,
			"ns":      1,
			"millis":  1,
			"command": 1,
		}
		opts := options.FindOne().SetSort(bson.D{{Key: "millis", Value: -1}}).SetProjection(projection)
		// Create a context with a timeout
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		var result bson.M
		err = collection.FindOne(ctx, filter, opts).Decode(&result)
		if err != nil {
			log.Println("Failed to find document for current hash: ", err)
			continue
		}

		// Extract the command field
		commandField, ok := result["command"].(bson.M)
		if !ok {
			log.Fatal("Failed to extract command field from document")
		}

		// Remove keys and replace values in the command field
		commandField = removeKeysAndReplace(commandField, []string{"projection", "lsid", "limit", "$db", "singleBatch", "find", "$clusterTime", "$readPreference"}, "redacted")

		// Update the result with the redacted command field
		result["command"] = commandField

		// Convert the result to JSON
		jsonResult, err := json.MarshalIndent(result, "", "    ")
		if err != nil {
			log.Fatal("Failed to marshal document to JSON: ", err)
		}

		// Print the result
		fmt.Println("Redacted Query:")
		fmt.Println(string(jsonResult))
		fmt.Println()

		// Prompts to use
		promptSystem := "Act as a devops/dba. Mongodb slow query from profiler will be provided to you. Answer only with index creation query without additional info. Using The ESR (Equality, Sort, Range) Rule create MongoDB index for profiler query. ESR: 'Equality' refers to an exact match on a single value; 'Sort' determines the order for results; 'Range' filters scan fields. The scan doesn't require an exact match. Additional Considerations: Inequality operators such as $ne or $nin are range operators, not equality operators. $regex is a range operator. When $in is used alone, it is an equality operator that performs a series of equality matches. When $in is used with .sort(), $in can act like a range operator"
		promptUser := string(jsonResult)

		switch *aiProvider {
		case "openai":
			// Set up OpenAI client
			openaiApiKey := *openaiApiKeyFlag
			if openaiApiKey == "" {
				log.Fatal("OpenAI API key is required")
			}
			openaiClient := openai.NewClient(openaiApiKey)

			ctxOpenai, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			response, err := askChatGPT(openaiClient, ctxOpenai, *openaiMaxTokens, promptSystem, promptUser)
			if err != nil {
				log.Fatal("Failed to get response from ChatGPT: ", err)
			}

			// Print the response from ChatGPT
			fmt.Println("Response from ChatGPT:")
			hyphens := strings.Repeat("-", 100)
			fmt.Println(hyphens)
			fmt.Println(response)
			fmt.Println(hyphens)
			fmt.Println()

		case "ollama":
			uri := "http://localhost:11434/v1/chat/completions"
			defer cancel()
			response, err := askOllama(uri, *ollamaModel, promptSystem, promptUser)
			if err != nil {
				log.Fatal("Failed to get response from Ollama: ", err)
			}

			// Print the response from Ollama
			fmt.Printf("Response from Ollama (%s):", *ollamaModel)
			fmt.Println()
			hyphens := strings.Repeat("-", 100)
			fmt.Println(hyphens)
			fmt.Println(response)
			fmt.Println(hyphens)
			fmt.Println()
		}
	}
}

func removeKeysAndReplace(query bson.M, keysToRemove []string, replaceValue interface{}) bson.M {
	for _, key := range keysToRemove {
		delete(query, key)
	}
	for k, v := range query {
		switch vt := v.(type) {
		case bson.M:
			query[k] = removeKeysAndReplace(vt, keysToRemove, replaceValue)
		default:
			query[k] = replaceValue
		}
	}
	return query
}

func askChatGPT(client *openai.Client, ctx context.Context, maxTokens int, promptSystem, promptUser string) (string, error) {
	resp, err := client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model:     openai.GPT3Dot5Turbo,
			MaxTokens: maxTokens,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: promptSystem,
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: promptUser,
				},
			},
		},
	)

	if err != nil {
		return "", err
	}

	return resp.Choices[0].Message.Content, nil
}

func askOllama(uri, model, promptSystem, promptUser string) (string, error) {
	// Define the request body
	requestBody, err := json.Marshal(map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": promptSystem},
			{"role": "user", "content": promptUser},
		},
	})
	if err != nil {
		fmt.Println("Error creating request body:", err)
		return "", err
	}

	// Make the HTTP POST request
	resp, err := http.Post(uri, "application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		fmt.Println("Error making request:", err)
		return "", err
	}
	defer resp.Body.Close()

	// Read the response body
	var response map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		fmt.Println("Error decoding response:", err)
		return "", err
	}

	var result string
	// Access and print the content of the first message in the choices array
	if choices, ok := response["choices"].([]interface{}); ok && len(choices) > 0 {
		if firstChoice, ok := choices[0].(map[string]interface{}); ok {
			if message, ok := firstChoice["message"].(map[string]interface{}); ok {
				if content, ok := message["content"].(string); ok {
					result = content
				}
			}
		}
	} else {
		fmt.Println("Failed to parse ollama response")
		return "", nil
	}

	return result, nil
}
