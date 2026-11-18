/*================================================================*/
/* CIERREDIA - TRABAJO NOCTURNO DEL CIERRE DIARIO BANCARIO         */
/*                                                                 */
/* ES EL PUNTO DE ENTRADA DEL BATCH EN LA IBM i: PREPARA LOS       */
/* ARCHIVOS DEL DIA, LLAMA AL PROGRAMA DIRECTOR BATCHDIA Y LANZA   */
/* EL EXTRACTO EN UN TRABAJO APARTE. SI ALGO FALLA, AVISA AL       */
/* OPERADOR Y SALE POR LA ETIQUETA ERROR.                          */
/*================================================================*/
PGM
             DCL        VAR(&FECHA)   TYPE(*CHAR) LEN(8)
             DCL        VAR(&USUARIO) TYPE(*CHAR) LEN(10)
             DCL        VAR(&TRABAJO) TYPE(*CHAR) LEN(10)
             DCL        VAR(&CODRET)  TYPE(*DEC)  LEN(2 0) VALUE(0)
             DCL        VAR(&MENSAJE) TYPE(*CHAR) LEN(80)

/* 1. DATOS DEL TRABAJO Y FECHA DE PROCESO                         */
             RTVJOBA    JOB(&TRABAJO) USER(&USUARIO)
             RTVSYSVAL  SYSVAL(QDATE) RTNVAL(&FECHA)
             CHGVAR     VAR(&MENSAJE) VALUE('CIERRE DIARIO ' *CAT +
                          &FECHA *CAT ' LANZADO POR ' *CAT &USUARIO)
             SNDPGMMSG  MSG(&MENSAJE) TOMSGQ(QSYSOPR)

/* 2. LOS NOMBRES LOGICOS DEL COBOL APUNTAN A LOS ARCHIVOS         */
/*    FISICOS DE LA BIBLIOTECA DE PRODUCCION (OVRDBF)              */
             OVRDBF     FILE(MOVIMIENTOS) TOFILE(BANCOLIB/MOVDIA) +
                          MBR(*FIRST)
             OVRDBF     FILE(CUENTAS) TOFILE(BANCOLIB/CTAMAE)
             MONMSG     MSGID(CPF0000) EXEC(GOTO CMDLBL(ERROR))

/* 3. PROGRAMA DIRECTOR DEL PROCESO NOCTURNO                       */
             CALL       PGM(BANCOLIB/BATCHDIA) PARM(&FECHA &CODRET)
             MONMSG     MSGID(CPF0000 MCH0000) EXEC(GOTO ERROR)

/* UN CODIGO DE RETORNO 16 DE BATCHDIA DETIENE EL CIERRE           */
             IF         COND(&CODRET *GE 16) THEN(DO)
                CHGVAR     VAR(&MENSAJE) VALUE('BATCHDIA TERMINO CON ' *CAT +
                             'CODIGO DE ERROR')
                GOTO       CMDLBL(ERROR)
             ENDDO

/* 4. EL EXTRACTO NO BLOQUEA EL CIERRE: SE LANZA COMO OTRO TRABAJO */
             SBMJOB     CMD(CALL PGM(EXTRACTO) PARM(&FECHA)) +
                          JOB(EXTRDIA) JOBQ(QBATCH)

/* 5. SE QUITAN LOS DESVIOS DE ARCHIVO Y SE TERMINA BIEN           */
 FIN:        DLTOVR     FILE(*ALL)
             MONMSG     MSGID(CPF0000)
             SNDPGMMSG  MSG('CIERRE DIARIO TERMINADO CORRECTAMENTE') +
                          TOMSGQ(QSYSOPR)
             RETURN

/* ERROR: AVISA AL OPERADOR Y LIBERA LOS ARCHIVOS                  */
 ERROR:      DLTOVR     FILE(*ALL)
             MONMSG     MSGID(CPF0000)
             SNDPGMMSG  MSG('CIERRE DIARIO CON ERRORES, REVISE ' *CAT +
                          &MENSAJE) TOMSGQ(QSYSOPR) MSGTYPE(*ESCAPE)
ENDPGM
