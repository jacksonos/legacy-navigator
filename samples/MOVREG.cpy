      * REGISTRO DE MOVIMIENTO
       01  MOV-REGISTRO.
           05  MOV-CUENTA          PIC 9(10).
           05  MOV-TIPO            PIC X.
               88  MOV-DEPOSITO    VALUE 'D'.
               88  MOV-RETIRO      VALUE 'R'.
           05  MOV-IMPORTE         PIC S9(9)V99.
           05  MOV-FECHA           PIC 9(8).
