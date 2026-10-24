package readings

import (
	"context"
	"errors"
	"math"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Repository accede a la colección "readings".
type Repository struct {
	coll *mongo.Collection
}

// NewRepository crea el repositorio sobre la base dada.
func NewRepository(db *mongo.Database) *Repository {
	return &Repository{coll: db.Collection("readings")}
}

// EnsureIndexes crea el índice compuesto y el TTL de 365 días sobre measuredAt.
func (r *Repository) EnsureIndexes(ctx context.Context) error {
	_, err := r.coll.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "cropId", Value: 1}, {Key: "measuredAt", Value: -1}},
			Options: options.Index().SetName("crop_measured"),
		},
		{
			Keys:    bson.D{{Key: "measuredAt", Value: 1}},
			Options: options.Index().SetExpireAfterSeconds(365 * 24 * 60 * 60),
		},
	})
	return err
}

// Insert guarda una lectura y fija su id generado.
func (r *Repository) Insert(ctx context.Context, reading *Reading) error {
	res, err := r.coll.InsertOne(ctx, reading)
	if err != nil {
		return err
	}
	if oid, ok := res.InsertedID.(primitive.ObjectID); ok {
		reading.ID = oid
	}
	return nil
}

// Latest devuelve la última lectura del cultivo, o nil si no tiene.
func (r *Repository) Latest(ctx context.Context, cropID primitive.ObjectID) (*Reading, error) {
	opts := options.FindOne().SetSort(bson.D{{Key: "measuredAt", Value: -1}})
	var reading Reading
	err := r.coll.FindOne(ctx, bson.M{"cropId": cropID}, opts).Decode(&reading)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &reading, nil
}

// Recent devuelve las últimas lecturas del cultivo, más recientes primero.
func (r *Repository) Recent(ctx context.Context, cropID primitive.ObjectID, limit int) ([]Reading, error) {
	opts := options.Find().SetSort(bson.D{{Key: "measuredAt", Value: -1}}).SetLimit(int64(limit))
	return r.find(ctx, bson.M{"cropId": cropID}, opts)
}

// Between devuelve las lecturas del cultivo en el rango, más recientes primero.
func (r *Repository) Between(ctx context.Context, cropID primitive.ObjectID, from, to time.Time, limit int) ([]Reading, error) {
	filter := bson.M{"cropId": cropID, "measuredAt": bson.M{"$gte": from, "$lte": to}}
	opts := options.Find().SetSort(bson.D{{Key: "measuredAt", Value: -1}}).SetLimit(int64(limit))
	return r.find(ctx, filter, opts)
}

// DeleteByCrop borra todas las lecturas de un cultivo.
func (r *Repository) DeleteByCrop(ctx context.Context, cropID primitive.ObjectID) error {
	_, err := r.coll.DeleteMany(ctx, bson.M{"cropId": cropID})
	return err
}

// Summary calcula el conteo y el mínimo, promedio y máximo por variable.
func (r *Repository) Summary(ctx context.Context, cropID primitive.ObjectID, from, to time.Time) (int64, map[string]MetricSummary, error) {
	group := bson.D{{Key: "_id", Value: nil}, {Key: "count", Value: bson.D{{Key: "$sum", Value: 1}}}}
	for _, m := range Metrics {
		field := "$measures." + m
		group = append(group,
			bson.E{Key: m + "Min", Value: bson.D{{Key: "$min", Value: field}}},
			bson.E{Key: m + "Avg", Value: bson.D{{Key: "$avg", Value: field}}},
			bson.E{Key: m + "Max", Value: bson.D{{Key: "$max", Value: field}}},
		)
	}
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.D{
			{Key: "cropId", Value: cropID},
			{Key: "measuredAt", Value: bson.D{{Key: "$gte", Value: from}, {Key: "$lte", Value: to}}},
		}}},
		{{Key: "$group", Value: group}},
	}
	cur, err := r.coll.Aggregate(ctx, pipeline)
	if err != nil {
		return 0, nil, err
	}
	var rows []bson.M
	if err := cur.All(ctx, &rows); err != nil {
		return 0, nil, err
	}
	metrics := make(map[string]MetricSummary)
	if len(rows) == 0 {
		return 0, metrics, nil
	}
	row := rows[0]
	count := toInt64(row["count"])
	for _, m := range Metrics {
		avg := toFloatPtr(row[m+"Avg"])
		if avg == nil {
			continue
		}
		rounded := math.Round(*avg*100) / 100
		metrics[m] = MetricSummary{Min: toFloatPtr(row[m+"Min"]), Avg: &rounded, Max: toFloatPtr(row[m+"Max"])}
	}
	return count, metrics, nil
}

// Bucket es el promedio de una variable de un cultivo en un intervalo.
type Bucket struct {
	CropID string
	Time   time.Time
	Value  float64
}

// SeriesAverages calcula promedios por cultivo e intervalo con $dateTrunc.
func (r *Repository) SeriesAverages(ctx context.Context, cropIDs []primitive.ObjectID, metric string, from time.Time, bucketMinutes int) ([]Bucket, error) {
	if len(cropIDs) == 0 {
		return nil, nil
	}
	field := "measures." + metric
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.D{
			{Key: "cropId", Value: bson.D{{Key: "$in", Value: cropIDs}}},
			{Key: "measuredAt", Value: bson.D{{Key: "$gte", Value: from}}},
			{Key: field, Value: bson.D{{Key: "$type", Value: "number"}}},
		}}},
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: bson.D{
				{Key: "cropId", Value: "$cropId"},
				{Key: "time", Value: bson.D{{Key: "$dateTrunc", Value: bson.D{
					{Key: "date", Value: "$measuredAt"},
					{Key: "unit", Value: "minute"},
					{Key: "binSize", Value: bucketMinutes},
				}}}},
			}},
			{Key: "value", Value: bson.D{{Key: "$avg", Value: "$" + field}}},
		}}},
		{{Key: "$sort", Value: bson.D{{Key: "_id.time", Value: 1}}}},
	}
	cur, err := r.coll.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		ID struct {
			CropID primitive.ObjectID `bson:"cropId"`
			Time   time.Time          `bson:"time"`
		} `bson:"_id"`
		Value float64 `bson:"value"`
	}
	if err := cur.All(ctx, &rows); err != nil {
		return nil, err
	}
	buckets := make([]Bucket, 0, len(rows))
	for _, row := range rows {
		buckets = append(buckets, Bucket{CropID: row.ID.CropID.Hex(), Time: row.ID.Time, Value: row.Value})
	}
	return buckets, nil
}

func (r *Repository) find(ctx context.Context, filter any, opts *options.FindOptions) ([]Reading, error) {
	cur, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	out := make([]Reading, 0)
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func toInt64(v any) int64 {
	switch n := v.(type) {
	case int32:
		return int64(n)
	case int64:
		return n
	case float64:
		return int64(n)
	default:
		return 0
	}
}

func toFloatPtr(v any) *float64 {
	switch n := v.(type) {
	case float64:
		return &n
	case int32:
		f := float64(n)
		return &f
	case int64:
		f := float64(n)
		return &f
	default:
		return nil
	}
}
