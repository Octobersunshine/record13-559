package main

import (
	"log"
	"net/http"
	"storedvalue/handler"
	"storedvalue/store"
)

func main() {
	s := store.NewMemoryStore()
	h := handler.NewHandler(s)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/balance", h.GetBalance)
	mux.HandleFunc("/api/deduct", h.Deduct)

	addr := ":8080"
	log.Printf("储值卡服务启动成功，监听地址: %s", addr)
	log.Printf("接口说明:")
	log.Printf("  GET  /api/balance?member_id=M001  查询会员储值卡余额")
	log.Printf("  POST /api/deduct                   消费扣款")
	log.Printf("测试会员 ID: M001(张三) M002(李四) M003(王五)")
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}
