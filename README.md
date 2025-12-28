# LogShipper – Kubernetes Node-Level Log Collector & S3 Archiver


<br><br>


A lightweight, high-performance Kubernetes DaemonSet that tails container logs directly from node filesystem, batches them per pod, compresses them, performs MD5-based deduplication, and uploads them safely to Amazon S3.

LogShipper is optimized for EKS, uses IRSA for authentication, supports multi-namespace log watching, and stores offsets to avoid data loss across pod/node restarts.