      *================================================================
      * MOVREG - REGISTRO DE MOVIMIENTO BANCARIO DEL DIA
      * UN REGISTRO POR OPERACION RECIBIDA DE LOS CANALES (VENTANILLA,
      * CAJERO, BANCA POR INTERNET, TRANSFERENCIAS INTERBANCARIAS)
      *================================================================
       01  MOV-REGISTRO.
           05  MOV-CUENTA          PIC 9(10).
           05  MOV-TIPO            PIC X.
               88  MOV-DEPOSITO    VALUE 'D'.
               88  MOV-RETIRO      VALUE 'R'.
               88  MOV-TRANSFER    VALUE 'T'.
               88  MOV-INTERES     VALUE 'I'.
               88  MOV-COMISION    VALUE 'C'.
               88  MOV-AJUSTE      VALUE 'A'.
           05  MOV-IMPORTE         PIC S9(9)V99.
           05  MOV-FECHA           PIC 9(8).
           05  MOV-CANAL           PIC XX.
               88  CANAL-VENTANILLA VALUE 'VE'.
               88  CANAL-CAJERO    VALUE 'CA'.
               88  CANAL-INTERNET  VALUE 'IN'.
               88  CANAL-INTERBANC VALUE 'IB'.
           05  MOV-REFERENCIA      PIC X(12).
           05  MOV-CTA-DESTINO     PIC 9(10).
           05  MOV-USUARIO         PIC X(8).
