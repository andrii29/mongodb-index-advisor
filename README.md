## MongoDB Index Advisor
This application is designed to enhance the efficiency of MongoDB database administrators (DBAs) and developers by automating the process of identifying slow queries, redacting sensitive information, and providing index suggestions using advanced AI capabilities.

### Features
- MongoDB Query Profiler Integration: Connects to your MongoDB instance and retrieves slow query data from the `system.profile` collection.
- Redaction of Sensitive Information: Analyzes the retrieved MongoDB queries, redacts sensitive information, and presents them for further analysis.
- AI-powered Index Suggestions: Leverages AI capabilities to generate index suggestions based on the extracted MongoDB queries, aiding in optimizing query performance. Currently supported AI providers: `OpenAi, Ollama`

### Prepare and Run
```bash
go mod tidy
go run . -h
```

### Usage Docker
```bash
# ollama
docker run --rm -it --net host andriik/mongodb-index-advisor -aiProvider ollama -dbName rto -mongoURI "mongodb://127.0.0.1:27017"
# openai
docker run --rm -it --net host andriik/mongodb-index-advisor -aiProvider openai -openaiApiKey "sk-token" -openaiMaxTokens 500 -dbName rto -mongoURI "mongodb://127.0.0.1:27017"
```

### Usage
```bash
  -aiProvider string
    	AI provider to use (ollama, openai) (default "openai")
  -dbName string
    	MongoDB database name (default "default")
  -millis int
    	Process queries with execution time >= millis
  -mongoURI string
    	MongoDB connection URI (default "mongodb://127.0.0.1:27017")
  -openaiApiKey string
    	OpenAI API key
  -openaiMaxTokens int
    	OpenAI maximum tokens per query (default 500)
```

## AI Providers
### OpenAI
- Create Api Key at [OpenAI Platform Site](https://platform.openai.com/account/api-keys) (Api Usage Is a Paid Service)

### Ollama
Ollama allows you to run open-source large language models and keep privacy. Setup:
```bash
curl -fsSL https://ollama.com/install.sh | sh
ollama run llama3
```

### MongoDB Authentication
To set up authentication, follow these steps:
```
mongosh

use admin
db.createUser({
  user: "mongodb-index-advisor",
  pwd: passwordPrompt(),
  roles: [ { role: "clusterMonitor", db: "admin" } ]
})

docker run --rm -it --net host andriik/mongodb-index-advisor -aiProvider ollama -dbName rto -mongoURI "mongodb://mongodb-profiler-exporter:<password>@127.0.0.1:27017/admin?authSource=admin&readPreference=primaryPreferred"
```

### Enable MongoDB Profiler
There are two ways to enable profiler in mongodb:
#### Per Dababase
```
use db_name
db.getProfilingStatus()
db.setProfilingLevel(1, { slowms: 100 })
```

#### Globally in mongod.conf
```yaml
operationProfiling:
  mode: slowOp
  slowOpThresholdMs: 50
```

### Increase system.profile size
The default size of the `system.profile` collection is set to 1MB, which can be insufficient for certain scenarios. To address this limitation, you can adjust the size of the collection by recreating it. Note that this process should not be replicated to replicas.

Below are example commands that can be used to increase the size of the system.profile collection:
```js
db.setProfilingLevel(0) // Disable profiling temporarily
db.system.profile.drop() // Drop the existing system.profile collection
db.createCollection( "system.profile", { capped: true, size: 1024 * 1024 * 50 } ) // 50Mb
db.setProfilingLevel(1, { slowms: 100 })  // Enable profiling again
```
