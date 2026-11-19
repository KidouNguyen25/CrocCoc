# V&V — Kế hoạch và đặc tả kiểm thử CrocCoc

| Mã | Phiên bản | Ngày | Trạng thái | Mã nguồn đối chiếu |
| --- | --- | --- | --- | --- |
| CC-VV-001 | 0.2 | 2026-11-19 | Kế hoạch, chưa nghiệm thu | `610b622` |

Tham chiếu [ISO/IEC/IEEE 29119-3:2021](https://www.iso.org/standard/79429.html). Kế hoạch này định nghĩa phạm vi, môi trường, dữ liệu, thủ tục, kết quả kỳ vọng và hồ sơ chạy. [Ma trận truy vết](traceability.md) ánh xạ từ yêu cầu qua hợp đồng tới các ca. Kết quả từ mock chỉ xác minh mock và wire shape, không nghiệm thu truyền thật.

## 1. Phạm vi và điều kiện vào

| Cấp | Đối tượng | Điều kiện chạy |
| --- | --- | --- |
| Đơn vị | Dịch vụ Go, mock, adapter TS | Chạy trên mã nguồn có SHA cụ thể. Backend độc lập không cần bundle, UI mock không cần Go. |
| Tích hợp | Go bridge, serialization, bundle frontend, binding native | Chạy full Go sau khi build frontend. Smoke Wails cần EXE Windows phù hợp và WebView2. |
| Nghiệm thu | Gửi/nhận Croc thật, hủy, lỗi, relay, toàn vẹn | Hai máy kiểm thử, Croc đúng bản pin, quyền đọc/ghi có kiểm soát, fixture không nhạy cảm, có phương tiện đối chiếu SHA-256. Ca relay cần hai mạng tách biệt và phải xác nhận đường truyền thực. |

Trước ca nghiệm thu, ghi Windows build, kiến trúc, WebView2, phiên bản app, SHA nguồn, phiên bản Croc, cấu hình mạng và danh sách fixture. Dùng một tệp nhỏ, một thư mục có tệp lồng nhau, một tên Unicode có khoảng trắng và một tệp lớn với số byte ghi trước khi chạy. Đích sạch cho ca mặc định, đích có quyền hạn hoặc tệp trùng tên cho ca biên. Đừng sử dụng thư mục cá nhân.

## 2. Ca khởi tạo và kiểm thử tự động

| Mã | Thủ tục | Kết quả kỳ vọng | Bằng chứng |
| --- | --- | --- | --- |
| CC-T-01 | `go test ./backend/...` | Dịch vụ stub báo unavailable, lệnh truyền/hủy không giả thành công và không phản chiếu mã. Serialize đúng `transferID`, trạng thái, thiếu `progress` và `progress:0`. | [Test dịch vụ Go](../backend/transfer/service_test.go), log thực thi. |
| CC-T-02 | `npm --prefix frontend test`. Sau build frontend, chạy `go test ./...`. | Mock đúng thứ tự và ID, từ chối phiên thứ hai, hủy không phát muộn. Adapter ánh xạ đối số và event, hủy listener. Go bridge giữ tham số Unicode, giá trị và lỗi. | [Test mock](../frontend/src/mockTransfer.test.mjs), [test adapter](../frontend/src/adapters/wailsTransfer.test.mjs), [test bridge](../app_test.go). |
| CC-T-03 | `npm --prefix frontend run build`, sau đó `wails build -platform windows/amd64`. | Frontend tạo bundle. Wails dùng bundle đó tạo EXE Windows x64 khi môi trường đầy đủ. Hai kết quả ghi riêng. | Log build, tên và SHA-256 artifact nếu có. |

## 3. Ca tích hợp và nghiệm thu mục tiêu

| Mã | Tiền điều kiện và bước chính | Kết quả đạt | Bằng chứng cần lưu |
| --- | --- | --- | --- |
| CC-T-11 | Thử bản pin, thiếu binary, sai phiên bản và binary không chạy được. Đọc `GetCrocInfo()` và thử bắt đầu với ca không khả dụng. | Chỉ binary đúng được báo khả dụng. Không có ID giả khi không khởi tạo được. Lỗi nêu hướng khắc phục. | Kết quả đã lọc đường dẫn, phiên bản, mã lỗi hoặc mô tả. |
| CC-T-12 | Chọn một tệp, sau đó chọn tập nhiều mục có thư mục Unicode, bỏ một mục, tạo mã và nhận vào thư mục đã chọn. Mở thư mục từ UI. | Mục bị bỏ không truyền. Mã sao chép dùng được. Đích đúng và danh sách, dung lượng, cấu trúc, SHA-256 khớp. Không lưu mã trong bằng chứng. | Manifest nguồn/đích và ảnh đã che mã. |
| CC-T-13 | Quan sát ID và trạng thái, thử mở phiên thứ hai, hủy khi chờ và khi đang truyền ở cả hai vai trò. Thử ID sai, hủy lại và bắt đầu phiên mới. | Một phiên hoạt động, sai ID không hủy nhầm. Một trạng thái cuối mỗi phiên. Không event muộn hoặc tiến trình con bị bỏ lại. Phiên mới được phép sau khi dọn xong. | Dòng thời gian theo ID, kiểm tra tiến trình, kết quả các lần hủy. |
| CC-T-14 | Thử mã sai, đích không ghi được, nguồn không còn, mất mạng và Croc thoát lỗi. Rà log, cấu hình, telemetry nếu có, đối số dòng lệnh, thông báo và báo cáo test. | Trước khi tạo phiên thì reject. Sau khi có phiên thì `FAILED`. UI vẫn phản hồi. Không tìm thấy mã kết nối, không có stderr thô trong thông báo công khai. | Bước tái hiện và kết quả rà soát đã lọc bí mật. |
| CC-T-15 | Dùng fixture đã ghi kích thước, truyền giữa hai mạng qua relay đã xác nhận. Theo dõi tương tác UI và nguồn số đo. | UI phản hồi, chỉ hiện tiến độ khi có dữ liệu tin cậy. Dung lượng, SHA-256 và cấu trúc khớp. | Bằng chứng relay không chứa mã, manifest nguồn/đích và log trạng thái đã lọc. |
| CC-T-16 | Mở đúng artifact trên Windows 10 và 11 x64 có WebView2. Xác minh mở UI và gọi binding cho bản Croc đi kèm. | Cả hai hệ điều hành mở được, UI tải và phiên bản Croc đúng bản pin. | SHA-256 artifact, bản Windows, WebView2 và kết quả binding. |

CC-T-11 đến CC-T-16 đều **chưa nghiệm thu** tại baseline này. [Thủ tục phát hành](release.md) dùng các ca này làm điều kiện bàn giao. Không đóng yêu cầu nếu ca bắt buộc chưa chạy, bị chặn hoặc thiếu bằng chứng.

## 4. Hồ sơ kết quả baseline và mẫu cho lần chạy sau

| Run | Môi trường và phạm vi | Kết quả tại lúc chốt init |
| --- | --- | --- |
| CC-RUN-INIT-01 | Windows phát triển, nguồn `610b622`, `npm --prefix frontend test` | **PASS** — 4/4 test mock và adapter, phần frontend của CC-T-02. |
| CC-RUN-INIT-02 | Cùng baseline, `npm --prefix frontend run build` | **PASS** — tạo bundle Vite, chỉ phần frontend của CC-T-03. |
| CC-RUN-INIT-03 | Cùng baseline, các bước cần Go/Wails | **BLOCKED / NOT RUN** — máy chạy không tìm thấy Go trên PATH. Không có kết quả Go, native EXE hoặc smoke từ lần chốt này. |

Mỗi lần chạy tiếp ghi ID run, ID ca, SHA hoặc SHA-256 artifact, ngày và người chạy, môi trường, dữ liệu mẫu, bước thực tế, kết quả quan sát, kết luận `PASS` / `FAIL` / `BLOCKED` / `NOT RUN` và liên kết bằng chứng hoặc lỗi. Tách rõ ca không chạy với ca chạy thất bại. Bằng chứng phải ẩn mã, tên tệp cá nhân và đường dẫn chứa danh tính.
