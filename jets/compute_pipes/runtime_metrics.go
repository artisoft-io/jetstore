package compute_pipes

import (
	"context"
	"log"
	"runtime"
)

func (ctx *BuilderContext) ReportMetrics() {
	stmt := `INSERT INTO jetsapi.cpipes_metrics (
		session_id, jets_partition, node_id, category, name, value, units) 
		VALUES ($1, $2, $3, $4, $5, $6, $7)`
	if ctx.cpConfig.MetricsConfig == nil {
		return
	}
	nodeId := ctx.nodeId
	var m runtime.MemStats
	f := float64(1024 * 1024)
	runtime.ReadMemStats(&m)
	for _, metric := range ctx.cpConfig.MetricsConfig.RuntimeMetrics {
		switch metric.Name {
		case "alloc_mb":
			_, err := ctx.dbpool.Exec(context.Background(), stmt, ctx.sessionId, ctx.jetsPartition, nodeId, metric.Type, metric.Name, float64(m.Alloc)/f, "MiB")
			if err != nil {
				log.Printf("error inserting in jetsapi.cpipes_metrics table: %v", err)
				return
			}
		case "total_alloc_mb":
			_, err := ctx.dbpool.Exec(context.Background(), stmt, ctx.sessionId, ctx.jetsPartition, nodeId, metric.Type, metric.Name, float64(m.TotalAlloc)/f, "MiB")
			if err != nil {
				log.Printf("error inserting in jetsapi.cpipes_metrics table: %v", err)
				return
			}
		case "sys_mb":
			_, err := ctx.dbpool.Exec(context.Background(), stmt, ctx.sessionId, ctx.jetsPartition, nodeId, metric.Type, metric.Name, float64(m.Sys)/f, "MiB")
			if err != nil {
				log.Printf("error inserting in jetsapi.cpipes_metrics table: %v", err)
				return
			}
		case "nbr_gc":
			_, err := ctx.dbpool.Exec(context.Background(), stmt, ctx.sessionId, ctx.jetsPartition, nodeId, metric.Type, metric.Name, float64(m.NumGC), "Count")
			if err != nil {
				log.Printf("error inserting in jetsapi.cpipes_metrics table: %v", err)
				return
			}
		}
	}
}
