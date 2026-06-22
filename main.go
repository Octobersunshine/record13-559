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
	mux.HandleFunc("/api/discount/config", h.GetDiscountConfig)
	mux.HandleFunc("/api/balance", h.GetBalance)
	mux.HandleFunc("/api/deduct", h.Deduct)

	addr := ":8080"
	log.Printf("储值卡服务启动成功，监听地址: %s", addr)
	log.Printf("接口说明:")
	log.Printf("  GET  /api/discount/config            查询全部折扣档位配置")
	log.Printf("  GET  /api/balance?member_id=M001     查询会员余额+当前档位+下一档差额")
	log.Printf("  POST /api/deduct                     消费扣款（按余额档位自动折扣）")
	log.Printf("默认折扣档位: 普通(无折扣)/白银(95折/≥1000元)/黄金(90折/≥3000元)/铂金(85折/≥5000元)/钻石(80折/≥10000元)")
	log.Printf("测试会员 ID: M001(张三 C001=5000元→铂金85折, C002=300.50元) M002(李四 1500元→白银95折) M003(王五 卡冻结)")
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}
