package simplemq_test

import (
	"context"
	"encoding/base64"
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/sacloud/sakumock/simplemq"
	"github.com/sacloud/simplemq-api-go/apis/v1/message"
)

func init() {
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func newBenchmarkServer(b *testing.B, database string) (*simplemq.Server, *message.Client) {
	b.Helper()
	srv := simplemq.NewTestServer(simplemq.Config{Database: database})
	client, err := message.NewClient(srv.TestURL(), &testSecuritySource{token: "bench"})
	if err != nil {
		b.Fatalf("failed to create client: %v", err)
	}
	return srv, client
}

func reportTPS(b *testing.B) {
	b.Helper()
	elapsed := b.Elapsed()
	if elapsed > 0 {
		tps := float64(b.N) / elapsed.Seconds()
		b.ReportMetric(tps, "tps")
	}
}

var benchContent = message.MessageContent(base64.StdEncoding.EncodeToString([]byte("hello")))

func benchmarkSend(b *testing.B, client *message.Client) {
	ctx := context.Background()
	params := message.SendMessageParams{QueueName: "bench-queue"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res, err := client.SendMessage(ctx, &message.SendRequest{Content: benchContent}, params)
		if err != nil {
			b.Fatal(err)
		}
		if _, ok := res.(*message.SendMessageOK); !ok {
			b.Fatalf("expected SendMessageOK, got %T", res)
		}
	}
	b.StopTimer()
	reportTPS(b)
}

func benchmarkSendReceiveDelete(b *testing.B, client *message.Client) {
	ctx := context.Background()
	queueName := message.QueueName("bench-queue")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sendRes, err := client.SendMessage(ctx, &message.SendRequest{Content: benchContent}, message.SendMessageParams{QueueName: queueName})
		if err != nil {
			b.Fatal(err)
		}
		sendOK, ok := sendRes.(*message.SendMessageOK)
		if !ok {
			b.Fatalf("expected SendMessageOK, got %T", sendRes)
		}

		recvRes, err := client.ReceiveMessage(ctx, message.ReceiveMessageParams{QueueName: queueName})
		if err != nil {
			b.Fatal(err)
		}
		if _, ok := recvRes.(*message.ReceiveMessageOK); !ok {
			b.Fatalf("expected ReceiveMessageOK, got %T", recvRes)
		}

		delRes, err := client.DeleteMessage(ctx, message.DeleteMessageParams{QueueName: queueName, MessageId: sendOK.Message.ID})
		if err != nil {
			b.Fatal(err)
		}
		if _, ok := delRes.(*message.DeleteMessageOK); !ok {
			b.Fatalf("expected DeleteMessageOK, got %T", delRes)
		}
	}
	b.StopTimer()
	reportTPS(b)
}

func benchmarkReceiveEmpty(b *testing.B, client *message.Client) {
	ctx := context.Background()
	params := message.ReceiveMessageParams{QueueName: "bench-queue"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res, err := client.ReceiveMessage(ctx, params)
		if err != nil {
			b.Fatal(err)
		}
		if _, ok := res.(*message.ReceiveMessageOK); !ok {
			b.Fatalf("expected ReceiveMessageOK, got %T", res)
		}
	}
	b.StopTimer()
	reportTPS(b)
}

func BenchmarkSend(b *testing.B) {
	b.Run("Memory", func(b *testing.B) {
		srv, client := newBenchmarkServer(b, "")
		defer srv.Close()
		benchmarkSend(b, client)
	})
	b.Run("SQLite", func(b *testing.B) {
		dbPath := filepath.Join(b.TempDir(), "bench.db")
		srv, client := newBenchmarkServer(b, dbPath)
		defer srv.Close()
		benchmarkSend(b, client)
	})
}

func BenchmarkSendReceiveDelete(b *testing.B) {
	b.Run("Memory", func(b *testing.B) {
		srv, client := newBenchmarkServer(b, "")
		defer srv.Close()
		benchmarkSendReceiveDelete(b, client)
	})
	b.Run("SQLite", func(b *testing.B) {
		dbPath := filepath.Join(b.TempDir(), "bench.db")
		srv, client := newBenchmarkServer(b, dbPath)
		defer srv.Close()
		benchmarkSendReceiveDelete(b, client)
	})
}

func BenchmarkReceiveEmpty(b *testing.B) {
	b.Run("Memory", func(b *testing.B) {
		srv, client := newBenchmarkServer(b, "")
		defer srv.Close()
		benchmarkReceiveEmpty(b, client)
	})
	b.Run("SQLite", func(b *testing.B) {
		dbPath := filepath.Join(b.TempDir(), "bench.db")
		srv, client := newBenchmarkServer(b, dbPath)
		defer srv.Close()
		benchmarkReceiveEmpty(b, client)
	})
}
