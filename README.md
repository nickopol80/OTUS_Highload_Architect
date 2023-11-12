# OTUS_Highload_Architect
Выполнение домашних заданий по программе OTUS Highload Architect

# Подготовительные работы:
- В БД MySQL необходимо создать 2е таблицы и настороить к ним доступ из проекта:
--1:
CREATE TABLE `articles` (
  `id` int unsigned NOT NULL AUTO_INCREMENT,
  `title` varchar(120) DEFAULT NULL,
  `anons` varchar(255) DEFAULT NULL,
  `text` text,
  `user_id` int unsigned DEFAULT NULL,
  PRIMARY KEY (`id`)
) ENGINE=MyISAM AUTO_INCREMENT=22 DEFAULT CHARSET=utf8mb3

--2:
CREATE TABLE `users` (
  `id` int unsigned NOT NULL AUTO_INCREMENT,
  `name` varchar(255) DEFAULT NULL,
  `age` int unsigned DEFAULT NULL,
  `surname` varchar(255) DEFAULT NULL,
  `sex` varchar(1) DEFAULT NULL,
  `city` varchar(255) DEFAULT NULL,
  `hobbies` text,
  `email` varchar(255) DEFAULT NULL,
  `password` varchar(50) DEFAULT NULL,
  PRIMARY KEY (`id`)
) ENGINE=MyISAM AUTO_INCREMENT=4 DEFAULT CHARSET=utf8mb3

# Запуск проекта:
1. Перейти в папку с проектом
2. Выполнить команду в терминале:
    go run .\main.go