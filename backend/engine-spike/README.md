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
go test -tags croc_no_tailcat -run TestWorkerLoopbackProjectsManifestAcrossPipes -v .
```

Worker nhận đúng một frame JSON có phiên bản từ stdin riêng, giới hạn 8 KiB và đợi EOF. Worker chỉ chấp nhận relay loopback dạng địa chỉ IP, tắt đường local discovery và không lấy mã từ argv hoặc env. Stdout dành cho đúng một kết quả có kiểu. `manifest_only` nghĩa là metadata hợp lệ và phiên đã dừng trước payload. Worker không chuyển source path, ID máy hoặc địa chỉ peer sang tiến trình gọi. Test Windows chạy sender qua relay TCP loopback và worker trong tiến trình con, kiểm danh sách Unicode và thư mục rỗng, không có mã hoặc mật khẩu trong kết quả và không có mục mới trong thư mục nhận.

Spike chưa giữ phiên chờ đồng ý, cấp phép tải, nối Croc vào staging, hoặc chạy trên hai máy Windows. Giới hạn giải mã upstream trước hook vẫn chưa phải ngân sách MVP. Đây không phải kênh production hay bằng chứng nghiệm thu relay công cộng. Host giữ `NOT_INTEGRATED`.
