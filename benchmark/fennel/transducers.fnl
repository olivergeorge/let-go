;; Transducer pipeline — no intermediate collections
(local core (require :io.gitlab.andreyorst.cljlib.core))

(core.transduce
  (core.comp (core.map #(* $ $))
             (core.filter core.even?)
             (core.take 100))
  core.+ 0
  (core.range 10000))
