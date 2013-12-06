#lang racket

;; Both `server' and `accept-and-handle' change
;; to use a custodian.

(define (serve port-no)
  (define main-cust (make-custodian))
  (parameterize ([current-custodian main-cust])
    (define listener (tcp-listen port-no 5 #t))
    (define (loop)
      (accept-and-handle listener)
      (loop))
    (thread loop))
  (lambda ()
    (custodian-shutdown-all main-cust)))

(define (accept-and-handle listener)
  (define cust (make-custodian))
  (parameterize ([current-custodian cust])
    (define-values (in out) (tcp-accept listener))
    (thread (lambda ()
              (handle in out)
              (close-input-port in)
              (close-output-port out))))
  ;; Watcher thread:
  ;;(thread (lambda ()
  ;;          (sleep 10)
  ;;          (custodian-shutdown-all cust)))
  )

(define (handle in out)
  ;; Discard the request header (up to blank line):
  (regexp-match #rx"(\r\n|^)\r\n" in)
  ;; Send reply:
  (display "HTTP/1.0 200 OK\r\n" out)
  (display "\r\n" out)
  (display buf out))

;1MB of data
(define buf (make-bytes (* 8 1024) 100))

;Start the server
(define stop (serve 8080))

;Sleep indefinitely
(sleep 500000000)