      *================================================================
      * AUDREG - AREA DE AUDITORIA QUE SE PASA A AUDITLOG
      * TODA RUTINA DEL BATCH REGISTRA SUS HITOS CON ESTE REGISTRO
      *================================================================
       01  AUD-REGISTRO.
           05  AUD-PROGRAMA        PIC X(8).
           05  AUD-ACCION          PIC X(10).
           05  AUD-FECHA           PIC 9(8).
           05  AUD-HORA            PIC 9(6).
           05  AUD-CUENTA          PIC 9(10).
           05  AUD-IMPORTE         PIC S9(11)V99.
           05  AUD-DETALLE         PIC X(40).
           05  AUD-RC              PIC 99.
               88  AUD-OK          VALUE 0.
