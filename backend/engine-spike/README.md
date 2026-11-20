# Spike nhận metadata của Croc

Bản thử ghim Croc v11.6.4 tại commit `f4cfdc17e06ac8a5834e146666798170138c3391`. Patch mượn giao thức và bộ giải mã Croc, không sửa wire. Áp dụng vào checkout upstream sạch để chạy lại. Không áp dụng vào checkout CrocCoc.

```text
git -C <croc-checkout> rev-parse HEAD
git -C <croc-checkout> apply --check <absolute-patch-path>
git -C <croc-checkout> apply <absolute-patch-path>
```

Trong checkout Croc đã vá, dùng toolchain theo `go.mod` upstream. Có thể biên dịch PE x64 trong Docker với `GOOS=windows GOARCH=amd64 CGO_ENABLED=0` rồi chạy test trên Windows.

```text
go test -c -o croc-manifest-probe.test.exe ./src/croc
croc-manifest-probe.test.exe -test.run TestCrocCocManifestOnly -test.v -test.timeout 25s
```

## Điểm dừng được xác minh

`ReceiveManifest` dùng client riêng và trả metadata có kiểu. Hook ở đầu `processSenderInfo`, trước khi chuẩn bị đầu ra hoặc đặt cờ cho phép yêu cầu dữ liệu. Dù callback trả `nil`, hook kết thúc bằng `ErrManifestOnly`, không phải thành công truyền tệp. Callback từ chối hoặc bị hủy không chuyển tiếp sang nhánh payload. API nhận thông thường của Croc không đổi.

Đã chạy PE x64 trên máy phát triển Windows. Bộ test kiểm frame legacy và chunked đã mã hóa, callback chỉ nhận danh sách đầy đủ, hủy hoặc từ chối, và luồng gửi/nhận qua relay TCP loopback. Ca loopback quan sát được metadata của một tệp, sender không vào trạng thái yêu cầu payload và thư mục nhận vẫn rỗng.

## Worker loopback

Module `worker/` nối callback Croc với bộ chiếu manifest của backend. Module thay thế nguồn Croc bằng checkout đã vá ở `.tmp/croc-engine-spike`. Từ `worker/`, build hoặc test với bản chỉ relay, không có Tailcat.

```text
go build -tags croc_no_tailcat .
go test -tags croc_no_tailcat -run TestWorkerLoopbackWaitsForDecisionWithoutStartingPayload -v .
```

Worker chỉ chấp nhận relay loopback dạng địa chỉ IP, tắt local discovery và không lấy mã từ argv hoặc env. Giao thức pipe riêng v2 dùng một giá trị JSON trên mỗi dòng, giới hạn mỗi frame đầu vào 8 KiB và không yêu cầu đóng stdin giữa các bước.

1. Tiến trình gọi gửi frame bắt đầu trên stdin. Worker trả `awaiting_decision` kèm manifest và `offerID` ngẫu nhiên trên stdout, rồi giữ nguyên phiên Croc chờ quyết định trong callback.
2. Tiến trình gọi gửi frame `{"version":2,"offerID":"<offer hiện tại>","accept":true}` hoặc `accept:false`. Quyết định phải đến trong thời hạn offer và không vượt quá deadline tổng của worker. Pipe bị đóng, sai offer hoặc sai frame đều dừng phiên.
3. Worker trả đúng một frame kết thúc. `accepted_no_payload` nghĩa là nhận được quyết định đồng ý nhưng **không** yêu cầu payload. Các trạng thái khác gồm `rejected`, `invalid_decision` và `cancelled`. Kết quả không chứa mã kết nối, mật khẩu, source path, ID máy hoặc địa chỉ peer.

Test loopback chạy sender và relay TCP, giữ stdin mở khi worker chờ, rồi kiểm các nhánh đồng ý, từ chối, sai offer và đóng pipe. Ca đồng ý vẫn không cho sender báo truyền thành công và không ghi vào thư mục nhận.

Spike đã giữ được phiên chờ đồng ý trong worker thử nghiệm. Chưa nối quyết định với permit của Coordinator, chưa nhận payload vào staging, và chưa nghiệm thu trên hai máy Windows. Giới hạn giải mã upstream trước hook vẫn chưa phải ngân sách MVP. Đây không phải kênh production hay bằng chứng nghiệm thu relay công cộng. Host giữ `NOT_INTEGRATED`.
