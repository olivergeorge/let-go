;; Map + filter + take pipeline over lazy seqs
(local core (require :io.gitlab.andreyorst.cljlib.core))

(core.reduce core.+ 0
  (core.take 100
    (core.filter core.even?
      (core.map #(* $ $) (core.range 10000)))))
