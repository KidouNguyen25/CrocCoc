# V&V — Kế hoạch kiểm thử và truy vết CrocCoc

| ID | Phiên bản | Trạng thái | Chủ trì |
| --- | --- | --- | --- |
| CC-VV-001 | 0.1 | Kế hoạch khởi tạo, chưa nghiệm thu bản đầu | Minh, Toàn review |

Tài liệu vận dụng [ISO/IEC/IEEE 29119-3:2021](https://www.iso.org/standard/79429.html) để ghi mục tiêu, môi trường, ca kiểm thử, tiêu chí đạt và bằng chứng. Mỗi lần chạy ghi phiên bản hoặc SHA, môi trường, lệnh/bước chạy, kết quả thực tế, bằng chứng đã lọc mã bí mật và ticket lỗi nếu có.

## Điều kiện kiểm thử

- Kiểm thử đơn vị có thể chạy riêng từng lớp. Go backend không cần frontend build; frontend sử dụng mock không cần Wails hoặc Croc.
- Kiểm thử Wails đầy đủ cần asset frontend đã build. Smoke native cần Windows 10/11 x64 và WebView2. Build chéo chỉ chứng minh build được, không thay cho smoke native.
- Kiểm thử truyền thật cần bản có Croc đúng phiên bản, hai máy kiểm thử, dữ liệu mẫu không nhạy cảm và khả năng kiểm tra SHA-256. Kiểm tra relay dùng hai mạng tách biệt.

## Kiểm thử khởi tạo

| Mã | Bước chính | Kết quả mong đợi |
| --- | --- | --- |
| CC-T-01 | Chạy `go test ./backend/...`. | Backend độc lập với Wails và frontend, báo chưa tích hợp, không phản chiếu mã nhận. JSON giữ đúng key và phân biệt tiến độ không có với 0%. |
| CC-T-02 | Chạy `npm --prefix frontend test`, build frontend rồi chạy `go test ./...`. | Mock phát đúng chuỗi event và dừng sau hủy. Adapter nhận đúng binding/event. Bridge Go chuyển tham số, giá trị trả về và lỗi nguyên trạng. |
| CC-T-03 | Chạy `npm --prefix frontend run build` và `wails build -platform windows/amd64`. | Build có bundle WebView và EXE Windows x64. Kết quả build tách biệt khỏi ca mở ứng dụng thật. |

## Ca nghiệm thu và ma trận truy vết

| Ca | Yêu cầu | Bước chính và kết quả mong đợi | Trạng thái |
| --- | --- | --- | --- |
| CC-T-11 | CC-FR-01, CC-NFR-02 | Thử binary hợp lệ, thiếu, sai phiên bản và không chạy được. Chỉ binary đã khóa được báo khả dụng; ca lỗi có hướng xử lý. | Chưa chạy, cần CC-02/07 |
| CC-T-12 | CC-FR-02, CC-FR-03, CC-FR-04, CC-NFR-03 | Gửi tệp đơn rồi nhiều tệp/thư mục có mục bị bỏ; sao chép mã và nhận vào thư mục đích. Mở đúng thư mục, hash/dung lượng/cấu trúc khớp. | Chưa chạy, cần CC-03/05/06 |
| CC-T-13 | CC-FR-05, CC-FR-06 | Thử tạo phiên thứ hai, hủy khi chờ và đang truyền ở cả hai vai trò. Không có tiến trình con bị bỏ lại và không có event sau trạng thái kết thúc. | Chưa chạy với Croc |
| CC-T-14 | CC-FR-04, CC-FR-07, CC-NFR-01 | Thử mã sai, đường dẫn lỗi, ngắt mạng và exit code khác 0. Rà output, log, cấu hình và dòng lệnh để bảo đảm không lộ mã nhận. | Chưa chạy với Croc |
| CC-T-15 | CC-FR-05, CC-NFR-03 | Truyền tệp lớn giữa hai mạng qua relay. Giao diện vẫn phản hồi, phần trăm chỉ xuất hiện nếu có số tin cậy, SHA-256 khớp. | Chưa chạy với Croc |
| CC-T-16 | CC-NFR-02 | Chạy EXE đóng gói trên Windows 10 và 11 x64, mở cửa sổ và gọi binding để đọc đúng phiên bản Croc đi kèm. | Chưa chạy native |

Điều kiện kết thúc phạm vi bản đầu là các ca `CC-T-11` đến `CC-T-16` đều có bằng chứng đạt, không còn lỗi chặn Gửi/Nhận, và README chạy được theo môi trường mục tiêu. Các ca chỉ dùng mock không được dùng để đóng yêu cầu truyền thật.
