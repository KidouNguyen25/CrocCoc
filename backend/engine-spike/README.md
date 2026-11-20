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

Spike này chưa cấp phép tải sau đồng ý, chưa nối bộ chiếu manifest hoặc staging vào client Croc, chưa chuyển secret qua pipe riêng và chưa xác minh hai máy Windows. Hook đang nhận `SenderInfo` thô trong engine. Adapter phải bỏ tuyệt đối trường nguồn, ID máy hay địa chỉ peer và chỉ đưa trường cho phép qua ranh giới tiến trình. Giới hạn giải mã trước hook vẫn là của Croc, chưa phải ngân sách MVP. Host giữ `NOT_INTEGRATED`.
