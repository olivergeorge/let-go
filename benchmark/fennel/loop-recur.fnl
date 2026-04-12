;; Tight loop with tail-call recursion
(import-macros {: loop} :io.gitlab.andreyorst.cljlib.core)

(loop [i 0 acc 0]
  (if (< i 1000000)
    (recur (+ i 1) (+ acc i))
    acc))
