# RTM — Truy vết yêu cầu

**CC-RTM-001 · 0.5 · 2026-11-19 · Kế hoạch.**

Yêu cầu trong [SRS](requirements.md), API trong [ICD](transfer-contract.md), ca trong [V&V](verification.md), thành phần trong [AD](architecture.md). Bảng là ánh xạ, không phải kết quả. Ghi nguồn chuẩn theo SRS hoặc quyết định thiết kế nội bộ. Notion chưa được đối chiếu nên không ánh xạ ticket lịch sử như nguồn đã xác minh.

| Yêu cầu | Thành phần hoặc điểm giao diện | Ca |
| --- | --- | --- |
| CC-FR-01 | ENGINE/HOST, CC-API-01 | CC-T-11 |
| CC-FR-02 | FE/HOST, CC-API-02/07 | CC-T-12A |
| CC-FR-03a | BE/FE, CC-API-05/06 | CC-T-13A, CC-T-14B |
| CC-FR-03b | HOST, CC-API-08 | CC-T-14B |
| CC-FR-04a | WIN-FS, CC-API-03 | CC-T-12B |
| CC-FR-04b | ENGINE/BE/FE, CC-API-05/06/10 | CC-T-11, CC-T-12B |
| CC-FR-04c | BE/WIN-FS, CC-API-03/10 | CC-T-12B |
| CC-FR-04d | BE/WIN-FS, CC-API-05 | CC-T-12B |
| CC-FR-04e | HOST, CC-API-09 | CC-T-12A |
| CC-FR-05a | HOST/BE, CC-API-02/03 | CC-T-13A, CC-T-13B |
| CC-FR-05b | BE/FE-IPC, CC-API-05/06 | CC-T-13A, CC-T-13B |
| CC-FR-05c | ENGINE/FE, CC-API-05/06 | CC-T-15A |
| CC-FR-05d | BE/ENGINE, CC-API-05/10 | CC-T-13A, CC-T-13B |
| CC-FR-06 | BE/ENGINE, CC-API-04/05 | CC-T-13A, CC-T-13B |
| CC-FR-07 | HOST/FE-IPC, CC-API-01/05 | CC-T-13A, CC-T-13B, CC-T-14A |
| CC-FR-08 | ENGINE/WIN-FS, CC-API-02 | CC-T-12A |
| CC-NFR-01 | HOST/ENGINE/FE, ngoại lệ kênh theo TM | CC-T-14A, CC-T-14B |
| CC-NFR-02 | HOST/ENGINE, gói phát hành | CC-T-11, CC-T-16A, CC-T-16B |
| CC-NFR-03 | ENGINE/WIN-FS, CC-API-05 | CC-T-12A, CC-T-12B, CC-T-15B |
| CC-NFR-04 | ENGINE/WIN-FS, cổng staging và quota | CC-T-12B |
| CC-NFR-05 | FE/HOST, bàn phím và thông báo | CC-T-13B |
| CC-NFR-06 | BE/HOST, xuất chẩn đoán | CC-T-14A |

CC-T-01–03 là kiểm baseline init, không thay ca mục tiêu. ID đã cấp không tái sử dụng.
