// Package db provides database connection pooling, transaction execution, and query abstractions
// for the SwiftDrop Core API.
//
// # Kubernetes Workload Scalability & Connection Pool Budget
//
// In accordance with Architecture Rule §4.4 and Decision D26, the database connection pool across
// all horizontally autoscaled stateless pods must obey the formula:
//
//	Replicas_max × MaxOpenConns ≤ PostgreSQL max_connections
//
// For example, with Core API scaled to a maximum of 3 replicas and MaxOpenConns = 10:
//
//	3 replicas × 10 max connections = 30 connections ≤ 100 (PostgreSQL standard limit).
//
// This guarantees that traffic spikes never exhaust available database backend processes or cause
// connection rejections for critical background worker and relay daemons.
package db
