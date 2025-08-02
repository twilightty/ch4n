// MongoDB initialization script for RegProxy
db = db.getSiblingDB('regproxy');

// Create the proxy collection
db.createCollection('proxy');

// Create indexes for optimal performance
db.proxy.createIndex({ "address": 1 }, { unique: true });
db.proxy.createIndex({ "is_working": -1, "last_tested": -1 });
db.proxy.createIndex({ "success_rate": -1, "latency_ms": 1 });
db.proxy.createIndex({ "updated_at": 1 }, { expireAfterSeconds: 604800 }); // 7 days TTL

// Create a user for the regproxy application
db.createUser({
  user: "regproxy",
  pwd: "regproxy123",
  roles: [
    {
      role: "readWrite",
      db: "regproxy"
    }
  ]
});

print("RegProxy MongoDB initialization completed");
print("Database: regproxy");
print("Collection: proxy");
print("User: regproxy");
print("Indexes created for optimal performance");
