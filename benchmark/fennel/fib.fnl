;; Tree-recursive fibonacci
(import-macros {: defn} :io.gitlab.andreyorst.cljlib.core)

(defn fib [n]
  (if (<= n 1)
    n
    (+ (fib (- n 1)) (fib (- n 2)))))

(fib 35)
