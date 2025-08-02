package storage

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ProxyDocument represents a proxy document in MongoDB
type ProxyDocument struct {
	ID          string    `bson:"_id,omitempty"`
	Address     string    `bson:"address"`
	IP          string    `bson:"ip"`
	Port        string    `bson:"port"`
	Type        string    `bson:"type"`
	Country     string    `bson:"country,omitempty"`
	Anonymity   string    `bson:"anonymity,omitempty"`
	IsWorking   bool      `bson:"is_working"`
	LastTested  time.Time `bson:"last_tested"`
	Latency     int64     `bson:"latency_ms"`
	TestCount   int       `bson:"test_count"`
	SuccessRate float64   `bson:"success_rate"`
	CreatedAt   time.Time `bson:"created_at"`
	UpdatedAt   time.Time `bson:"updated_at"`
}

// MongoStorage handles MongoDB operations for proxy storage
type MongoStorage struct {
	client     *mongo.Client
	database   *mongo.Database
	collection *mongo.Collection
	logger     *log.Logger
}

// NewMongoStorage creates a new MongoDB storage instance
func NewMongoStorage(dsn, database, collection string, timeout time.Duration, logger *log.Logger) (*MongoStorage, error) {
	// Set client options
	clientOptions := options.Client().ApplyURI(dsn)
	clientOptions.SetConnectTimeout(timeout)
	clientOptions.SetServerSelectionTimeout(timeout)

	// Connect to MongoDB
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %v", err)
	}

	// Test the connection
	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("failed to ping MongoDB: %v", err)
	}

	db := client.Database(database)
	coll := db.Collection(collection)

	// Create indexes
	storage := &MongoStorage{
		client:     client,
		database:   db,
		collection: coll,
		logger:     logger,
	}

	if err := storage.createIndexes(ctx); err != nil {
		logger.Printf("Warning: Failed to create indexes: %v", err)
	}

	logger.Printf("Connected to MongoDB: %s/%s.%s", dsn, database, collection)
	return storage, nil
}

// createIndexes creates necessary indexes for the collection
func (m *MongoStorage) createIndexes(ctx context.Context) error {
	// Index on address (unique)
	addressIndex := mongo.IndexModel{
		Keys:    bson.D{bson.E{Key: "address", Value: 1}},
		Options: options.Index().SetUnique(true),
	}

	// Index on is_working and last_tested
	workingIndex := mongo.IndexModel{
		Keys: bson.D{
			bson.E{Key: "is_working", Value: -1},
			bson.E{Key: "last_tested", Value: -1},
		},
	}

	// Index on success_rate and latency_ms
	performanceIndex := mongo.IndexModel{
		Keys: bson.D{
			bson.E{Key: "success_rate", Value: -1},
			bson.E{Key: "latency_ms", Value: 1},
		},
	}

	// TTL index on updated_at (remove old non-working proxies after 7 days)
	ttlIndex := mongo.IndexModel{
		Keys:    bson.D{bson.E{Key: "updated_at", Value: 1}},
		Options: options.Index().SetExpireAfterSeconds(7 * 24 * 3600), // 7 days
	}

	_, err := m.collection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		addressIndex,
		workingIndex,
		performanceIndex,
		ttlIndex,
	})

	return err
}

// SaveWorkingProxies saves or updates working proxy results
func (m *MongoStorage) SaveWorkingProxies(ctx context.Context, results []ProxyTestResult) error {
	if len(results) == 0 {
		return nil
	}

	now := time.Now()
	operations := make([]mongo.WriteModel, 0, len(results))

	for _, result := range results {
		doc := ProxyDocument{
			Address:    result.Address,
			IP:         result.IP,
			Port:       result.Port,
			Type:       result.Type,
			IsWorking:  result.IsWorking,
			LastTested: now,
			Latency:    result.Latency.Milliseconds(),
			UpdatedAt:  now,
		}

		// For working proxies, increment test count and update success rate
		if result.IsWorking {
			filter := bson.M{"address": result.Address}

			// Calculate new success rate
			updateWithSuccessRate := bson.M{
				"$set": bson.M{
					"ip":          doc.IP,
					"port":        doc.Port,
					"type":        doc.Type,
					"is_working":  doc.IsWorking,
					"last_tested": doc.LastTested,
					"latency_ms":  doc.Latency,
					"updated_at":  doc.UpdatedAt,
				},
				"$inc": bson.M{
					"test_count": 1,
				},
				"$setOnInsert": bson.M{
					"created_at": now,
				},
			}

			operation := mongo.NewUpdateOneModel().
				SetFilter(filter).
				SetUpdate(updateWithSuccessRate).
				SetUpsert(true)

			operations = append(operations, operation)
		} else {
			// For non-working proxies, just update the status
			filter := bson.M{"address": result.Address}
			update := bson.M{
				"$set": bson.M{
					"is_working":  false,
					"last_tested": now,
					"updated_at":  now,
				},
				"$inc": bson.M{
					"test_count": 1,
				},
				"$setOnInsert": bson.M{
					"ip":           doc.IP,
					"port":         doc.Port,
					"type":         doc.Type,
					"created_at":   now,
					"success_rate": 0.0,
				},
			}

			operation := mongo.NewUpdateOneModel().
				SetFilter(filter).
				SetUpdate(update).
				SetUpsert(true)

			operations = append(operations, operation)
		}
	}

	// Execute bulk write
	opts := options.BulkWrite().SetOrdered(false)
	result, err := m.collection.BulkWrite(ctx, operations, opts)
	if err != nil {
		return fmt.Errorf("failed to bulk write proxies: %v", err)
	}

	m.logger.Printf("MongoDB: Processed %d proxies (Inserted: %d, Modified: %d, Upserted: %d)",
		len(results), result.InsertedCount, result.ModifiedCount, result.UpsertedCount)

	// Update success rates
	return m.updateSuccessRates(ctx)
}

// updateSuccessRates calculates and updates success rates for all proxies
func (m *MongoStorage) updateSuccessRates(ctx context.Context) error {
	// Aggregate pipeline to calculate success rates
	pipeline := []bson.M{
		{
			"$match": bson.M{
				"test_count": bson.M{"$gt": 0},
			},
		},
		{
			"$addFields": bson.M{
				"success_rate": bson.M{
					"$cond": bson.M{
						"if":   bson.M{"$eq": []interface{}{"$is_working", true}},
						"then": 1.0,
						"else": 0.0,
					},
				},
			},
		},
		{
			"$merge": bson.M{
				"into": m.collection.Name(),
				"on":   "_id",
				"whenMatched": bson.M{
					"$set": bson.M{
						"success_rate": "$success_rate",
					},
				},
			},
		},
	}

	_, err := m.collection.Aggregate(ctx, pipeline)
	return err
}

// GetWorkingProxies retrieves working proxies from MongoDB
func (m *MongoStorage) GetWorkingProxies(ctx context.Context, limit int) ([]string, error) {
	filter := bson.M{
		"is_working": true,
		"last_tested": bson.M{
			"$gte": time.Now().Add(-24 * time.Hour), // Only proxies tested in last 24 hours
		},
	}

	opts := options.Find().
		SetSort(bson.D{
			{Key: "success_rate", Value: -1},
			{Key: "latency_ms", Value: 1},
		}).
		SetLimit(int64(limit))

	cursor, err := m.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to query working proxies: %v", err)
	}
	defer cursor.Close(ctx)

	var proxies []string
	for cursor.Next(ctx) {
		var doc ProxyDocument
		if err := cursor.Decode(&doc); err != nil {
			m.logger.Printf("Error decoding proxy document: %v", err)
			continue
		}
		proxies = append(proxies, doc.Address)
	}

	return proxies, cursor.Err()
}

// GetProxyStats returns statistics about stored proxies
func (m *MongoStorage) GetProxyStats(ctx context.Context) (map[string]interface{}, error) {
	pipeline := []bson.M{
		{
			"$group": bson.M{
				"_id":   nil,
				"total": bson.M{"$sum": 1},
				"working": bson.M{
					"$sum": bson.M{
						"$cond": []interface{}{"$is_working", 1, 0},
					},
				},
				"avg_latency": bson.M{
					"$avg": bson.M{
						"$cond": []interface{}{"$is_working", "$latency_ms", nil},
					},
				},
				"avg_success_rate": bson.M{"$avg": "$success_rate"},
			},
		},
	}

	cursor, err := m.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to get proxy stats: %v", err)
	}
	defer cursor.Close(ctx)

	var result map[string]interface{}
	if cursor.Next(ctx) {
		if err := cursor.Decode(&result); err != nil {
			return nil, fmt.Errorf("failed to decode stats: %v", err)
		}
	}

	// Add timestamp
	result["timestamp"] = time.Now()
	return result, nil
}

// CleanupOldProxies removes old non-working proxies
func (m *MongoStorage) CleanupOldProxies(ctx context.Context, maxAge time.Duration) error {
	filter := bson.M{
		"is_working": false,
		"updated_at": bson.M{
			"$lt": time.Now().Add(-maxAge),
		},
	}

	result, err := m.collection.DeleteMany(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to cleanup old proxies: %v", err)
	}

	if result.DeletedCount > 0 {
		m.logger.Printf("Cleaned up %d old non-working proxies", result.DeletedCount)
	}

	return nil
}

// Close closes the MongoDB connection
func (m *MongoStorage) Close(ctx context.Context) error {
	return m.client.Disconnect(ctx)
}

// ProxyTestResult represents the result of testing a proxy
type ProxyTestResult struct {
	Address   string
	IP        string
	Port      string
	Type      string
	IsWorking bool
	Latency   time.Duration
	Error     error
}
