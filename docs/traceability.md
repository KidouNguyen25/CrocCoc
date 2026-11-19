# RTM — Ma trận truy vết yêu cầu CrocCoc

| Mã | Phiên bản | Ngày | Trạng thái | Mã nguồn đối chiếu |
| --- | --- | --- | --- | --- |
| CC-RTM-001 | 0.2 | 2026-11-19 | Dự thảo để duyệt | `610b622` |

Đọc bảng từ trái sang phải để xác định cần đổi gì khi yêu cầu thay đổi. Đọc ngược để biết một ca kiểm thử chứng minh yêu cầu nào. Mã yêu cầu nằm trong [SRS](requirements.md), mã giao diện trong [ICD](transfer-contract.md), ca và kết quả trong [V&V](verification.md). FE, FE-IPC, HOST, BE và CLI là các thành phần trong [kiến trúc](architecture.md). Ánh xạ đến CLI mô tả đích tích hợp, không khẳng định đã có runner.

| Yêu cầu | Nguồn | Thành phần | Giao diện liên quan | Ca nghiệm thu mục tiêu |
| --- | --- | --- | --- | --- |
| CC-FR-01 | CC-02 | FE, FE-IPC, HOST, BE, CLI | CC-API-01 | CC-T-11 |
| CC-FR-02 | CC-03, CC-05 | FE, BE, CLI | CC-API-02, chọn/bỏ mục là hành vi UI | CC-T-12 |
| CC-FR-03 | CC-03, CC-05 | FE, FE-IPC, HOST, BE, CLI | CC-API-02, CC-API-05 | CC-T-12 |
| CC-FR-04 | CC-03, CC-06 | FE, FE-IPC, HOST, BE, CLI | CC-API-03, CC-API-05. Mở thư mục chưa có binding trong init. | CC-T-12, CC-T-14 |
| CC-FR-05 | CC-01, CC-03–CC-06 | FE, FE-IPC, HOST, BE | CC-API-02, CC-API-03, CC-API-04, CC-API-05 | CC-T-13, CC-T-15 |
| CC-FR-06 | CC-03, CC-05, CC-06 | FE, FE-IPC, HOST, BE, CLI | CC-API-04, CC-API-05 | CC-T-13 |
| CC-FR-07 | CC-03, CC-08 | FE, FE-IPC, HOST, BE, CLI | CC-API-02, CC-API-03, CC-API-05 | CC-T-14 |
| CC-NFR-01 | CC-02, CC-03, CC-08 | FE, HOST, BE, CLI | CC-API-02, CC-API-03, CC-API-05 | CC-T-14 |
| CC-NFR-02 | CC-07, CC-08 | HOST, BE, CLI | CC-API-01 và bản phân phối | CC-T-11, CC-T-16 |
| CC-NFR-03 | CC-08 | BE, CLI | CC-API-02, CC-API-03, CC-API-05 | CC-T-12, CC-T-15 |

CC-T-01/02/03 là kiểm tra nền tảng. Các ca này hỗ trợ đánh giá biên dữ liệu, khả năng build và hành vi mẫu, nhưng không thay thế cột nghiệm thu.

Khi đổi hoặc thêm yêu cầu, giữ nguyên ID đã cấp, cập nhật nguồn và tiêu chí chấp nhận, rà tác động API và thiết kế, rồi bổ sung ca và bằng chứng tương ứng. Khi bỏ yêu cầu, đánh dấu không còn áp dụng và lưu lý do thay vì cấp lại ID cho nội dung khác.
